package assetpool

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

// AssetAPI exposes the asset_* JSON-RPC namespace for the Asset Pool.
// Methods are automatically exported by go-ethereum's RPC framework using
// reflection. Method names map to RPC calls as: asset_<MethodName> (lowercased first letter).
type AssetAPI struct {
	pool *AssetPool
}

// NewAssetAPI creates a new RPC API service backed by the given asset pool.
func NewAssetAPI(pool *AssetPool) *AssetAPI {
	return &AssetAPI{pool: pool}
}

// UploadAsset uploads a new media asset to the pool via RPC.
// dataHex is the hex-encoded raw media content.
func (api *AssetAPI) UploadAsset(title, description, creator, signature, contentType, dataHex string) (map[string]interface{}, error) {
	dataHex = strings.TrimPrefix(dataHex, "0x")
	data, err := hex.DecodeString(dataHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex data: %w", err)
	}

	meta := AssetMetadata{
		Title:       title,
		Description: description,
		Creator:     creator,
		Signature:   signature,
		ContentType: contentType,
	}

	assetId, err := api.pool.StoreAsset(data, meta)
	if err != nil {
		return nil, err
	}

	storedMeta, _ := api.pool.GetMetadata(assetId)

	return map[string]interface{}{
		"assetId":  assetId,
		"verified": storedMeta != nil && storedMeta.Verified,
	}, nil
}

// GetAsset returns the hex-encoded binary data and metadata for an asset.
func (api *AssetAPI) GetAsset(assetId string) (map[string]interface{}, error) {
	data, meta, err := api.pool.GetAsset(assetId)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"data":     hexutil.Encode(data),
		"metadata": meta,
	}, nil
}

// GetMetadata returns just the metadata for an asset.
func (api *AssetAPI) GetMetadata(assetId string) (*AssetMetadata, error) {
	return api.pool.GetMetadata(assetId)
}

// ListAssets returns all active (non-expired) assets in the pool.
func (api *AssetAPI) ListAssets() []AssetListEntry {
	return api.pool.ListAssets()
}

// MarkPurchased records a purchaser on the asset in the node's pool.
func (api *AssetAPI) MarkPurchased(assetId, purchaser string) (bool, error) {
	if err := api.pool.MarkPurchased(assetId, purchaser); err != nil {
		return false, err
	}
	return true, nil
}

// ConfirmReceived marks an asset as received by the purchaser.
func (api *AssetAPI) ConfirmReceived(assetId, purchaser string) (bool, error) {
	if err := api.pool.ConfirmReceived(assetId, purchaser); err != nil {
		return false, err
	}
	return true, nil
}

// Prune manually triggers an eviction sweep and returns the number of removed assets.
func (api *AssetAPI) Prune() (int, error) {
	count := api.pool.Prune()
	return count, nil
}

// GetStats returns aggregate pool statistics.
func (api *AssetAPI) GetStats() (PoolStats, error) {
	return api.pool.GetStats(), nil
}
