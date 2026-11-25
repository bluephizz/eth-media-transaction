package assetpool

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// VerifyCreatorSignature verifies that the given signature over the assetId
// (a 32-byte Keccak-256 hash) was produced by the claimed creator address.
//
// It supports two signing schemes:
//  1. EIP-191 personal_sign: "\x19Ethereum Signed Message:\n32" + assetId
//  2. Raw 32-byte hash signing (direct ecrecover on assetId)
func VerifyCreatorSignature(assetIdHex string, signatureHex string, creatorHex string) bool {
	assetIdBytes := common.FromHex(assetIdHex)
	sigBytes := common.FromHex(signatureHex)
	creatorAddr := common.HexToAddress(creatorHex)

	if len(assetIdBytes) != 32 || len(sigBytes) != 65 {
		return false
	}

	// Normalize v value: Ethereum signatures have v = 27 or 28, but
	// crypto.Ecrecover expects v = 0 or 1.
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	// Try EIP-191 personal_sign prefix first
	prefixed := crypto.Keccak256(
		[]byte("\x19Ethereum Signed Message:\n32"),
		assetIdBytes,
	)
	pubKey, err := crypto.Ecrecover(prefixed, sigBytes)
	if err == nil && len(pubKey) > 0 {
		recovered := common.BytesToAddress(crypto.Keccak256(pubKey[1:])[12:])
		if recovered == creatorAddr {
			return true
		}
	}

	// Fallback: try raw hash
	pubKey, err = crypto.Ecrecover(assetIdBytes, sigBytes)
	if err == nil && len(pubKey) > 0 {
		recovered := common.BytesToAddress(crypto.Keccak256(pubKey[1:])[12:])
		if recovered == creatorAddr {
			return true
		}
	}

	return false
}
