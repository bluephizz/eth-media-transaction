package assetpool

import "time"

// AssetStatus represents the lifecycle state of a media asset in the pool.
type AssetStatus string

const (
	StatusActive    AssetStatus = "ACTIVE"
	StatusPurchased AssetStatus = "PURCHASED"
	StatusReceived  AssetStatus = "RECEIVED"
	StatusExpired   AssetStatus = "EXPIRED"
)

// AssetMetadata holds the JSON-serializable metadata for an asset stored in the pool.
type AssetMetadata struct {
	ID                  string      `json:"id"`
	Title               string      `json:"title"`
	Description         string      `json:"description"`
	ContentType         string      `json:"contentType"`
	Size                int64       `json:"size"`
	Creator             string      `json:"creator"`
	Signature           string      `json:"signature"`
	Verified            bool        `json:"verified"`
	Status              AssetStatus `json:"status"`
	Purchaser           string      `json:"purchaser,omitempty"`
	DesignatedPurchaser string      `json:"designatedPurchaser,omitempty"`
	Price               string      `json:"price,omitempty"`
	CreatedAt           time.Time   `json:"createdAt"`
	ExpiresAt           time.Time   `json:"expiresAt"`
	PurchasedAt         *time.Time  `json:"purchasedAt,omitempty"`
	ReceivedAt          *time.Time  `json:"receivedAt,omitempty"`
}

// SecondsRemaining returns the number of seconds until the asset expires.
// Returns 0 if the asset has already expired.
func (m *AssetMetadata) SecondsRemaining() float64 {
	remaining := time.Until(m.ExpiresAt).Seconds()
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsExpired returns true if the asset's retention window has passed.
func (m *AssetMetadata) IsExpired() bool {
	return time.Now().After(m.ExpiresAt)
}

// AssetListEntry is a JSON-serializable summary entry returned by the list endpoint.
type AssetListEntry struct {
	ID               string      `json:"id"`
	Title            string      `json:"title"`
	ContentType      string      `json:"contentType"`
	Size             int64       `json:"size"`
	Creator          string      `json:"creator"`
	Status           AssetStatus `json:"status"`
	Verified         bool        `json:"verified"`
	SecondsRemaining float64     `json:"secondsRemaining"`
}

// PoolStats holds aggregate statistics for the asset pool.
type PoolStats struct {
	ActiveAssets    int     `json:"activeAssets"`
	TotalBytes      int64   `json:"totalBytes"`
	RetentionWindow string  `json:"retentionWindow"`
}
