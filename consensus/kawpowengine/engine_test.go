//go:build cgo
// +build cgo

package kawpowengine

import (
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/kawpow"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params/types/goethereum"
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

func sealedEngineHeader(t testing.TB) *types.Header {
	t.Helper()
	header := &types.Header{
		ParentHash:  common.HexToHash("0x01"),
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    common.HexToAddress("0x1234"),
		Root:        common.HexToHash("0x02"),
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		Difficulty:  big.NewInt(2),
		Number:      big.NewInt(42),
		GasLimit:    30_000_000,
		Time:        1_700_000_000,
		Extra:       []byte("aichain-kawpow-engine"),
	}
	input, err := kawpow.HeaderToVerificationInput(header)
	if err != nil {
		t.Fatal(err)
	}
	var headerHash [32]byte
	copy(headerHash[:], input.SealHash[:])
	for nonce := uint64(0); ; nonce++ {
		mix, final, err := kawpow.C1Hash(int(input.Height), headerHash, nonce)
		if err != nil {
			t.Fatal(err)
		}
		if final[0]&0x80 == 0 {
			header.Nonce = types.EncodeNonce(nonce)
			header.MixDigest = common.Hash(mix)
			return header
		}
	}
}

func TestVerifySealAcceptsValidAndRejectsTampering(t *testing.T) {
	e := New(ethash.Config{})
	header := sealedEngineHeader(t)
	if err := e.VerifySeal(header); err != nil {
		t.Fatalf("valid seal rejected: %v", err)
	}

	tamperedMix := types.CopyHeader(header)
	tamperedMix.MixDigest[0]++
	if err := e.VerifySeal(tamperedMix); err == nil {
		t.Fatal("tampered mix accepted")
	}

	tamperedNonce := types.CopyHeader(header)
	tamperedNonce.Nonce = types.EncodeNonce(header.Nonce.Uint64() + 1)
	if err := e.VerifySeal(tamperedNonce); err == nil {
		t.Fatal("tampered nonce accepted")
	}

	tamperedDifficulty := types.CopyHeader(header)
	tamperedDifficulty.Difficulty = big.NewInt(3)
	if err := e.VerifySeal(tamperedDifficulty); err == nil {
		t.Fatal("tampered difficulty accepted")
	}
}

func TestVerifyHeaderHonoursSealFlag(t *testing.T) {
	e := New(ethash.Config{PowMode: ethash.ModeFullFake})
	header := sealedEngineHeader(t)
	if err := e.VerifyHeader(nil, header, true); err != nil {
		t.Fatalf("valid sealed header rejected: %v", err)
	}

	tampered := types.CopyHeader(header)
	tampered.MixDigest[0]++
	if err := e.VerifyHeader(nil, tampered, false); err != nil {
		t.Fatalf("seal-disabled structural check rejected test header: %v", err)
	}
	if err := e.VerifyHeader(nil, tampered, true); err == nil {
		t.Fatal("seal-enabled header check accepted tampered KawPoW mix")
	}
}

func TestVerifyHeadersPreservesResultOrder(t *testing.T) {
	e := New(ethash.Config{PowMode: ethash.ModeFullFake})
	valid := sealedEngineHeader(t)
	tampered := types.CopyHeader(valid)
	tampered.MixDigest[0]++

	_, results := e.VerifyHeaders(nil, []*types.Header{valid, tampered, valid}, []bool{true, true, true})
	got := make([]error, 0, 3)
	for err := range results {
		got = append(got, err)
	}
	if len(got) != 3 {
		t.Fatalf("received %d results, want 3", len(got))
	}
	if got[0] != nil || got[1] == nil || got[2] != nil {
		t.Fatalf("unexpected ordered results: [%v, %v, %v]", got[0], got[1], got[2])
	}
}

func TestVerifySealConcurrent(t *testing.T) {
	e := New(ethash.Config{})
	header := sealedEngineHeader(t)
	const workers = 16
	const iterations = 4

	var wait sync.WaitGroup
	errors := make(chan error, workers*iterations)
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				errors <- e.VerifySeal(header)
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent valid seal rejected: %v", err)
		}
	}
}

func TestSealHashUsesCandidateMapping(t *testing.T) {
	e := New(ethash.Config{})
	h := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(2)}
	if got, want := e.SealHash(h), kawpow.SealHash(h); got != want {
		t.Fatalf("SealHash = %x, want %x", got, want)
	}
}

func TestDevelopmentDifficultyMatchesCoreGethBaseline(t *testing.T) {
	config := &goethereum.ChainConfig{}
	parent := &types.Header{
		Number:     big.NewInt(1),
		Time:       1_000,
		Difficulty: big.NewInt(2_000_000),
	}
	for _, timestamp := range []uint64{1_001, 1_010, 1_100} {
		got := DevelopmentCalcDifficulty(config, timestamp, parent)
		want := ethash.CalcDifficulty(config, timestamp, parent)
		if got.Cmp(want) != 0 {
			t.Fatalf("time %d: difficulty = %s, want %s", timestamp, got, want)
		}
	}
}
