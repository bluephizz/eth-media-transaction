// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Package assetpool implements an ephemeral in-node media asset storage pool
// for 2-way artwork transactions on Ethereum. Media files are persisted for a
// configurable retention window (default 48 hours), verified via secp256k1
// signatures, and automatically pruned after expiry or delivery acknowledgement.
package assetpool

import "time"

// Config holds the configuration for the Asset Pool subsystem.
type Config struct {
	// Datadir is the subdirectory under the node's instance directory where
	// media asset files are stored on disk.
	Datadir string

	// RetentionWindow is the duration that an asset will be kept in the pool
	// before being eligible for automatic pruning. Default is 48 hours.
	RetentionWindow time.Duration

	// Enabled indicates whether the asset pool subsystem is active.
	Enabled bool
}

// DefaultConfig contains the default configuration values for the Asset Pool.
var DefaultConfig = Config{
	Datadir:         "assetpool",
	RetentionWindow: 48 * time.Hour,
	Enabled:         true,
}
