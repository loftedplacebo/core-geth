// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.

package kawpow

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// EpochLength is the KawPoW DAG epoch length used by cpp-kawpow.
const EpochLength uint64 = 7500

// SeedHash returns the DAG seed for the epoch containing blockNumber.
func SeedHash(blockNumber uint64) common.Hash {
	seed := make([]byte, common.HashLength)
	for epoch := uint64(0); epoch < blockNumber/EpochLength; epoch++ {
		seed = crypto.Keccak256(seed)
	}
	return common.BytesToHash(seed)
}
