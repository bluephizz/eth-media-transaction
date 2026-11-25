package assetpool

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func signEIP191(t *testing.T, privHex string, hash []byte) string {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privHex, "0x"))
	if err != nil {
		t.Fatalf("invalid key: %v", err)
	}
	prefixed := crypto.Keccak256([]byte("\x19Ethereum Signed Message:\n32"), hash)
	sig, err := crypto.Sign(prefixed, key)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	sig[64] += 27
	return hexutil.Encode(sig)
}

func signRaw(t *testing.T, privHex string, hash []byte) string {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privHex, "0x"))
	if err != nil {
		t.Fatalf("invalid key: %v", err)
	}
	sig, err := crypto.Sign(hash, key)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	sig[64] += 27
	return hexutil.Encode(sig)
}

func TestAssetPoolLifecycleAndPruning(t *testing.T) {
	const (
		aliceAddr = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
		aliceKey  = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
		bobAddr   = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
	)

	tmpDir := t.TempDir()
	pool, err := New(Config{
		Datadir:         "assetpool",
		RetentionWindow: 48 * time.Hour,
		Enabled:         true,
	}, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	payload := []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"><rect width=\"10\" height=\"10\"/></svg>")
	hash := crypto.Keccak256(payload)
	sig := signEIP191(t, aliceKey, hash)

	// Spoofed creator should be rejected
	if _, err := pool.StoreAsset(payload, AssetMetadata{
		Title:     "Spoofed",
		Creator:   bobAddr,
		Signature: sig,
	}); err == nil {
		t.Fatalf("expected error for spoofed creator signature")
	}

	// Valid creator signature should succeed
	assetId, err := pool.StoreAsset(payload, AssetMetadata{
		Title:       "Aurora Test",
		Description: "Unit test SVG",
		ContentType: "image/svg+xml",
		Creator:     aliceAddr,
		Signature:   sig,
	})
	if err != nil {
		t.Fatalf("StoreAsset failed: %v", err)
	}

	// Verify disk files exist
	if _, err := os.Stat(filepath.Join(tmpDir, "assetpool", assetId+".bin")); err != nil {
		t.Fatalf("missing bin file on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "assetpool", assetId+".json")); err != nil {
		t.Fatalf("missing json file on disk: %v", err)
	}

	// Verify reload from disk in a fresh pool instance
	pool2, err := New(Config{
		Datadir:         filepath.Join(tmpDir, "assetpool"), // also test absolute datadir
		RetentionWindow: 48 * time.Hour,
		Enabled:         true,
	}, tmpDir)
	if err != nil {
		t.Fatalf("New (reload) failed: %v", err)
	}
	gotData, gotMeta, err := pool2.GetAsset(strings.ToUpper(assetId))
	if err != nil {
		t.Fatalf("GetAsset failed: %v", err)
	}
	if !bytes.Equal(gotData, payload) {
		t.Fatalf("payload mismatch")
	}
	if !gotMeta.Verified || gotMeta.Status != StatusActive {
		t.Fatalf("unexpected metadata: %+v", gotMeta)
	}

	// Mark purchased & received, then prune
	if err := pool2.MarkPurchased(assetId, bobAddr); err != nil {
		t.Fatalf("MarkPurchased failed: %v", err)
	}
	if err := pool2.ConfirmReceived(assetId, bobAddr); err != nil {
		t.Fatalf("ConfirmReceived failed: %v", err)
	}
	if pruned := pool2.Prune(); pruned != 1 {
		t.Fatalf("expected 1 pruned asset, got %d", pruned)
	}
	if _, _, err := pool2.GetAsset(assetId); err == nil {
		t.Fatalf("expected asset to be deleted after pruning")
	}
}

func TestAssetPoolRawSignatureAndHTTP(t *testing.T) {
	const (
		aliceAddr = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
		aliceKey  = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	)

	tmpDir := t.TempDir()
	pool, err := New(DefaultConfig, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	payload := []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")
	hash := crypto.Keccak256(payload)
	rawSig := signRaw(t, aliceKey, hash)

	// Upload via HTTP multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("title", "HTTP SVG")
	_ = writer.WriteField("creator", aliceAddr)
	_ = writer.WriteField("signature", rawSig)
	part, err := writer.CreateFormFile("file", "art.svg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(payload)
	writer.Close()

	handler := pool.HTTPHandler()
	req := httptest.NewRequest(http.MethodPost, "/asset/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload returned %d: %s", rec.Code, rec.Body.String())
	}

	var uploadResp struct {
		Success  bool   `json:"success"`
		AssetID  string `json:"assetId"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatal(err)
	}
	if !uploadResp.Success || !uploadResp.Verified {
		t.Fatalf("unexpected upload response: %+v", uploadResp)
	}

	// GET /asset/{id}
	getReq := httptest.NewRequest(http.MethodGet, "/asset/"+uploadResp.AssetID, nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get asset returned %d", getRec.Code)
	}
	if getRec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("expected image/svg+xml, got %q", getRec.Header().Get("Content-Type"))
	}
	if getRec.Header().Get("X-Asset-Verified") != "true" {
		t.Fatalf("expected X-Asset-Verified=true")
	}

	// GET /asset/stats
	statsReq := httptest.NewRequest(http.MethodGet, "/asset/stats", nil)
	statsRec := httptest.NewRecorder()
	handler.ServeHTTP(statsRec, statsReq)
	if statsRec.Code != http.StatusOK {
		t.Fatalf("stats returned %d", statsRec.Code)
	}

	// Test RPC API wrapper
	api := NewAssetAPI(pool)
	stats, err := api.GetStats()
	if err != nil || stats.ActiveAssets != 1 {
		t.Fatalf("unexpected RPC stats: %+v, err=%v", stats, err)
	}
}
