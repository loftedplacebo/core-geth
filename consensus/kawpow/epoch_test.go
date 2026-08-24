// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.

package kawpow

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestSeedHashEpochBoundaries(t *testing.T) {
	zero := common.Hash{}
	one := common.BytesToHash(crypto.Keccak256(zero[:]))
	two := common.BytesToHash(crypto.Keccak256(one[:]))
	if want := common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"); one != want {
		t.Fatalf("epoch 1 seed = %s, want cpp-kawpow vector %s", one, want)
	}
	tests := []struct {
		block uint64
		want  common.Hash
	}{
		{0, zero},
		{EpochLength - 1, zero},
		{EpochLength, one},
		{2*EpochLength - 1, one},
		{2 * EpochLength, two},
	}
	for _, test := range tests {
		if got := SeedHash(test.block); got != test.want {
			t.Fatalf("SeedHash(%d) = %s, want %s", test.block, got, test.want)
		}
	}
}
