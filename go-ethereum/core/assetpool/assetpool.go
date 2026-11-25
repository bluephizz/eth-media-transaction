package assetpool

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

// AssetPool is the core manager for the ephemeral media asset storage subsystem.
// It stores media files on disk indexed by their Keccak-256 hash and maintains
// an in-memory registry of metadata for fast lookups.
type AssetPool struct {
	config  Config
	datadir string
	mu      sync.RWMutex
	assets  map[string]*AssetMetadata // normalized lowercase 0x-prefixed assetId -> metadata
	quit    chan struct{}
	running bool
}

// normalizeAssetID normalizes an asset ID hex string to lowercase with a 0x prefix.
func normalizeAssetID(id string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	if id != "" && !strings.HasPrefix(id, "0x") {
		id = "0x" + id
	}
	return id
}

// New creates a new AssetPool instance. The instanceDir is the node's data
// directory. If cfg.Datadir is absolute it is used directly; otherwise media
// files are stored under instanceDir/cfg.Datadir/.
func New(cfg Config, instanceDir string) (*AssetPool, error) {
	if cfg.Datadir == "" {
		cfg.Datadir = DefaultConfig.Datadir
	}
	if cfg.RetentionWindow <= 0 {
		cfg.RetentionWindow = DefaultConfig.RetentionWindow
	}

	var datadir string
	if filepath.IsAbs(cfg.Datadir) {
		datadir = cfg.Datadir
	} else {
		datadir = filepath.Join(instanceDir, cfg.Datadir)
	}

	if err := os.MkdirAll(datadir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create asset pool directory %s: %w", datadir, err)
	}

	pool := &AssetPool{
		config:  cfg,
		datadir: datadir,
		assets:  make(map[string]*AssetMetadata),
		quit:    make(chan struct{}),
	}

	// Reload any existing assets from disk
	if err := pool.loadFromDisk(); err != nil {
		log.Warn("Asset pool: error loading existing assets from disk", "err", err)
	}

	log.Info("Asset Pool initialized",
		"datadir", datadir,
		"retention", cfg.RetentionWindow,
		"loaded", len(pool.assets),
	)
	return pool, nil
}

// Start begins the background pruning goroutine.
func (ap *AssetPool) Start() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.running {
		return
	}
	ap.quit = make(chan struct{})
	ap.running = true
	go ap.pruneLoop()
	log.Info("Asset Pool background pruner started", "interval", "5m")
}

// Stop halts the background pruning goroutine.
func (ap *AssetPool) Stop() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if !ap.running {
		return
	}
	close(ap.quit)
	ap.running = false
	log.Info("Asset Pool stopped")
}

// HTTPHandler returns an http.Handler that serves the Asset Pool HTTP API
// mounted at /asset/ on the Geth HTTP server.
func (ap *AssetPool) HTTPHandler() http.Handler {
	return newHTTPHandler(ap)
}

// StoreAsset persists a media asset to disk and registers it in the memory index.
// It computes the Keccak-256 hash of the data, verifies the creator's secp256k1
// signature, and rejects unverified or spoofed payloads.
func (ap *AssetPool) StoreAsset(data []byte, meta AssetMetadata) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty asset payload")
	}

	// Compute the keccak256 hash of the raw media content
	hash := crypto.Keccak256Hash(data)
	assetId := normalizeAssetID(hash.Hex())

	if meta.ID != "" && normalizeAssetID(meta.ID) != assetId {
		return "", fmt.Errorf("asset ID mismatch: expected %s, got %s", assetId, meta.ID)
	}
	meta.ID = assetId

	if meta.Creator == "" || meta.Signature == "" {
		return "", fmt.Errorf("creator address and signature are required")
	}

	// Verify the creator's secp256k1 signature against keccak256(content)
	if !VerifyCreatorSignature(assetId, meta.Signature, meta.Creator) {
		log.Warn("Asset Pool: creator signature verification failed",
			"assetId", assetId[:16]+"...",
			"creator", meta.Creator,
		)
		return "", fmt.Errorf("invalid creator signature for asset %s", assetId)
	}
	meta.Verified = true

	// Set timestamps
	now := time.Now()
	meta.CreatedAt = now
	meta.ExpiresAt = now.Add(ap.config.RetentionWindow)
	meta.Size = int64(len(data))
	if meta.Status == "" {
		meta.Status = StatusActive
	}

	// Write binary data to disk
	binPath := filepath.Join(ap.datadir, assetId+".bin")
	if err := os.WriteFile(binPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write asset binary: %w", err)
	}

	// Write metadata to disk
	metaPath := filepath.Join(ap.datadir, assetId+".json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		os.Remove(binPath)
		return "", fmt.Errorf("failed to marshal asset metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		os.Remove(binPath)
		return "", fmt.Errorf("failed to write asset metadata: %w", err)
	}

	// Register in memory
	ap.mu.Lock()
	ap.assets[assetId] = &meta
	ap.mu.Unlock()

	log.Info("Asset Pool: stored new asset",
		"id", assetId[:16]+"...",
		"title", meta.Title,
		"size", meta.Size,
		"creator", meta.Creator,
		"verified", meta.Verified,
		"expiresAt", meta.ExpiresAt.Format(time.RFC3339),
	)

	return assetId, nil
}

// GetAsset returns the binary content of an asset and its metadata.
func (ap *AssetPool) GetAsset(assetId string) ([]byte, *AssetMetadata, error) {
	assetId = normalizeAssetID(assetId)

	ap.mu.RLock()
	meta, ok := ap.assets[assetId]
	ap.mu.RUnlock()

	if !ok {
		return nil, nil, fmt.Errorf("asset not found: %s", assetId)
	}

	binPath := filepath.Join(ap.datadir, assetId+".bin")
	data, err := os.ReadFile(binPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read asset binary: %w", err)
	}

	return data, meta, nil
}

// GetMetadata returns just the metadata for an asset.
func (ap *AssetPool) GetMetadata(assetId string) (*AssetMetadata, error) {
	assetId = normalizeAssetID(assetId)

	ap.mu.RLock()
	meta, ok := ap.assets[assetId]
	ap.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("asset not found: %s", assetId)
	}
	return meta, nil
}

// ListAssets returns a list of all active (non-expired) assets in the pool.
func (ap *AssetPool) ListAssets() []AssetListEntry {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	entries := make([]AssetListEntry, 0, len(ap.assets))
	for _, meta := range ap.assets {
		if meta.IsExpired() {
			continue
		}
		entries = append(entries, AssetListEntry{
			ID:               meta.ID,
			Title:            meta.Title,
			ContentType:      meta.ContentType,
			Size:             meta.Size,
			Creator:          meta.Creator,
			Status:           meta.Status,
			Verified:         meta.Verified,
			SecondsRemaining: meta.SecondsRemaining(),
		})
	}
	return entries
}

// MarkPurchased records a purchaser on the asset.
func (ap *AssetPool) MarkPurchased(assetId, purchaser string) error {
	assetId = normalizeAssetID(assetId)

	ap.mu.Lock()
	defer ap.mu.Unlock()

	meta, ok := ap.assets[assetId]
	if !ok {
		return fmt.Errorf("asset not found: %s", assetId)
	}

	now := time.Now()
	meta.Status = StatusPurchased
	meta.Purchaser = purchaser
	meta.PurchasedAt = &now

	return ap.persistMetadata(meta)
}

// ConfirmReceived marks an asset as received by the purchaser.
func (ap *AssetPool) ConfirmReceived(assetId, purchaser string) error {
	assetId = normalizeAssetID(assetId)

	ap.mu.Lock()
	defer ap.mu.Unlock()

	meta, ok := ap.assets[assetId]
	if !ok {
		return fmt.Errorf("asset not found: %s", assetId)
	}

	now := time.Now()
	meta.Status = StatusReceived
	if purchaser != "" {
		meta.Purchaser = purchaser
	}
	meta.ReceivedAt = &now

	return ap.persistMetadata(meta)
}

// GetStats returns aggregate pool statistics.
func (ap *AssetPool) GetStats() PoolStats {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	var totalBytes int64
	activeCount := 0
	for _, meta := range ap.assets {
		if !meta.IsExpired() {
			activeCount++
			totalBytes += meta.Size
		}
	}

	return PoolStats{
		ActiveAssets:    activeCount,
		TotalBytes:      totalBytes,
		RetentionWindow: ap.config.RetentionWindow.String(),
	}
}

// Prune manually triggers an eviction sweep and returns the count of removed assets.
func (ap *AssetPool) Prune() int {
	return ap.pruneExpired()
}

// pruneExpired removes all expired and acknowledged-received assets from disk and memory.
func (ap *AssetPool) pruneExpired() int {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	var removed int
	for id, meta := range ap.assets {
		if meta.IsExpired() || meta.Status == StatusReceived {
			// Remove from disk
			binPath := filepath.Join(ap.datadir, id+".bin")
			metaPath := filepath.Join(ap.datadir, id+".json")
			os.Remove(binPath)
			os.Remove(metaPath)

			delete(ap.assets, id)
			removed++

			log.Info("Asset Pool: pruned asset",
				"id", id[:16]+"...",
				"status", meta.Status,
				"expired", meta.IsExpired(),
			)
		}
	}

	if removed > 0 {
		log.Info("Asset Pool: prune cycle complete", "removed", removed, "remaining", len(ap.assets))
	}
	return removed
}

// pruneLoop runs in the background, periodically sweeping for expired assets.
func (ap *AssetPool) pruneLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ap.pruneExpired()
		case <-ap.quit:
			return
		}
	}
}

// loadFromDisk scans the data directory for existing .json metadata files and
// loads them into the in-memory index.
func (ap *AssetPool) loadFromDisk() error {
	entries, err := os.ReadDir(ap.datadir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		metaPath := filepath.Join(ap.datadir, entry.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			log.Warn("Asset Pool: failed to read metadata file", "path", metaPath, "err", err)
			continue
		}

		var meta AssetMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			log.Warn("Asset Pool: failed to parse metadata file", "path", metaPath, "err", err)
			continue
		}

		meta.ID = normalizeAssetID(meta.ID)
		// Verify the binary file also exists
		binPath := filepath.Join(ap.datadir, meta.ID+".bin")
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			log.Warn("Asset Pool: metadata without binary, skipping", "id", meta.ID)
			continue
		}

		ap.assets[meta.ID] = &meta
	}
	return nil
}

// persistMetadata writes updated metadata back to disk (called under lock).
func (ap *AssetPool) persistMetadata(meta *AssetMetadata) error {
	metaPath := filepath.Join(ap.datadir, meta.ID+".json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}
