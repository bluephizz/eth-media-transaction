package assetpool

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

// httpHandler implements http.Handler for the Asset Pool HTTP endpoints.
type httpHandler struct {
	pool *AssetPool
}

func newHTTPHandler(pool *AssetPool) http.Handler {
	return &httpHandler{pool: pool}
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for browser-based access
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Expose-Headers", "X-Asset-Creator, X-Asset-Verified, X-Asset-Status, X-Asset-Expires-At")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Strip the /asset/ prefix to get the sub-path
	subPath := strings.TrimPrefix(r.URL.Path, "/asset/")
	subPath = strings.TrimPrefix(subPath, "/asset")
	subPath = strings.TrimLeft(subPath, "/")

	switch {
	case r.Method == http.MethodPost && (subPath == "upload" || subPath == "upload/"):
		h.handleUpload(w, r)
	case (r.Method == http.MethodGet || r.Method == http.MethodHead) && (subPath == "list" || subPath == "list/"):
		h.handleList(w, r)
	case (r.Method == http.MethodGet || r.Method == http.MethodHead) && (subPath == "stats" || subPath == "stats/"):
		h.handleStats(w, r)
	case (r.Method == http.MethodGet || r.Method == http.MethodHead) && len(subPath) > 0:
		h.handleGetAsset(w, r, subPath)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

// detectContentType determines the MIME type of the uploaded media payload.
func detectContentType(headerType string, data []byte) string {
	if headerType != "" && headerType != "application/octet-stream" {
		return headerType
	}
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("<svg")) || (bytes.HasPrefix(trimmed, []byte("<?xml")) && bytes.Contains(trimmed, []byte("<svg"))) {
		return "image/svg+xml"
	}
	return http.DetectContentType(data)
}

// handleUpload processes POST /asset/upload
// Accepts multipart/form-data with fields: title, description, creator, signature, price, file
func (h *httpHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	var fileData []byte
	var meta AssetMetadata

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form (max 50MB)
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse multipart form: " + err.Error()})
			return
		}

		meta.Title = r.FormValue("title")
		meta.Description = r.FormValue("description")
		meta.Creator = r.FormValue("creator")
		meta.Signature = r.FormValue("signature")
		meta.Price = r.FormValue("price")
		meta.DesignatedPurchaser = r.FormValue("designatedPurchaser")

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field required: " + err.Error()})
			return
		}
		defer file.Close()

		fileData, err = io.ReadAll(file)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read file: " + err.Error()})
			return
		}

		meta.ContentType = detectContentType(header.Header.Get("Content-Type"), fileData)
	} else if strings.HasPrefix(contentType, "application/json") {
		// JSON upload: {"title":"...", "creator":"...", "signature":"...", "data":"0x...", "contentType":"..."}
		var req struct {
			Title               string `json:"title"`
			Description         string `json:"description"`
			Creator             string `json:"creator"`
			Signature           string `json:"signature"`
			Price               string `json:"price"`
			DesignatedPurchaser string `json:"designatedPurchaser"`
			Data                string `json:"data"` // hex-encoded
			ContentType         string `json:"contentType"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse JSON body: " + err.Error()})
			return
		}

		meta.Title = req.Title
		meta.Description = req.Description
		meta.Creator = req.Creator
		meta.Signature = req.Signature
		meta.Price = req.Price
		meta.DesignatedPurchaser = req.DesignatedPurchaser

		if req.Data != "" {
			decoded, err := hex.DecodeString(strings.TrimPrefix(req.Data, "0x"))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid hex data: " + err.Error()})
				return
			}
			fileData = decoded
		}
		meta.ContentType = detectContentType(req.ContentType, fileData)
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported content type, use multipart/form-data or application/json"})
		return
	}

	if len(fileData) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty file data"})
		return
	}

	assetId, err := h.pool.StoreAsset(fileData, meta)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "signature") || strings.Contains(err.Error(), "creator") || strings.Contains(err.Error(), "mismatch") || strings.Contains(err.Error(), "empty") {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	storedMeta, _ := h.pool.GetMetadata(assetId)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"assetId":  assetId,
		"verified": storedMeta != nil && storedMeta.Verified,
		"expiresAt": func() string {
			if storedMeta != nil {
				return storedMeta.ExpiresAt.Format(time.RFC3339)
			}
			return ""
		}(),
	})
}

// handleGetAsset serves GET/HEAD /asset/{assetId} — streams the binary content
func (h *httpHandler) handleGetAsset(w http.ResponseWriter, r *http.Request, assetId string) {
	assetId = normalizeAssetID(path.Base(assetId))

	data, meta, err := h.pool.GetAsset(assetId)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	// Set custom verification headers
	w.Header().Set("X-Asset-Creator", meta.Creator)
	w.Header().Set("X-Asset-Verified", strconv.FormatBool(meta.Verified))
	w.Header().Set("X-Asset-Status", string(meta.Status))
	w.Header().Set("X-Asset-Expires-At", meta.ExpiresAt.Format(time.RFC3339))

	// Set content type
	ct := detectContentType(meta.ContentType, data)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Header().Set("Cache-Control", "no-cache")

	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		w.Write(data)
	}
}

// handleList serves GET /asset/list
func (h *httpHandler) handleList(w http.ResponseWriter, r *http.Request) {
	entries := h.pool.ListAssets()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"assets": entries,
		"count":  len(entries),
	})
}

// handleStats serves GET /asset/stats
func (h *httpHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.pool.GetStats()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"stats": stats,
	})
}

// writeJSON serializes a value to JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
