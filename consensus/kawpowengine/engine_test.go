//go:build cgo
// +build cgo

package kawpowengine

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/kawpow"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestDevelopmentEngineDisablesMining(t *testing.T) {
	e := New(ethash.Config{})
	if err := e.Seal(nil, nil, nil, nil); err != ErrMiningUnavailable {
		t.Fatalf("Seal error = %v", err)
	}
	if e.Hashrate() != 0 {
		t.Fatal("development engine reports a hashrate")
	}
}

func TestVerifySealRejectsMalformedCandidateInputs(t *testing.T) {
	e := New(ethash.Config{})

	if err := e.VerifySeal(nil); err != kawpow.ErrNilHeader {
		t.Fatalf("nil header error = %v, want %v", err, kawpow.ErrNilHeader)
	}
	if err := e.VerifySeal(&types.Header{Number: big.NewInt(1)}); err != kawpow.ErrInvalidDifficulty {
		t.Fatalf("missing difficulty error = %v, want %v", err, kawpow.ErrInvalidDifficulty)
	}
	if err := e.VerifySeal(&types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(1)}); err != kawpow.ErrDifficultyBelowMinimum {
		t.Fatalf("difficulty-one error = %v, want %v", err, kawpow.ErrDifficultyBelowMinimum)
	}
	if err := e.VerifySeal(&types.Header{Number: new(big.Int).SetUint64(1 << 31), Difficulty: big.NewInt(2)}); err != kawpow.ErrUnsupportedBlockNumber {
		t.Fatalf("height-range error = %v, want %v", err, kawpow.ErrUnsupportedBlockNumber)
	}
}

func TestSealHashUsesCandidateMapping(t *testing.T) {
	e := New(ethash.Config{})
	h := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(2)}
	if got, want := e.SealHash(h), kawpow.SealHash(h); got != want {
		t.Fatalf("SealHash = %x, want %x", got, want)
	}
}
