package kawpow

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func candidateHeader() *types.Header {
	return &types.Header{
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
		Extra:       []byte("aichain-c1-header"),
	}
}

func sealedCandidateHeader(t testing.TB) *types.Header {
	t.Helper()
	header := candidateHeader()
	input, err := HeaderToVerificationInput(header)
	if err != nil {
		t.Fatal(err)
	}
	var headerHash [32]byte
	copy(headerHash[:], input.SealHash[:])
	for nonce := uint64(0); ; nonce++ {
		mix, final, err := C1Hash(int(input.Height), headerHash, nonce)
		if err != nil {
			t.Fatal(err)
		}
		// Difficulty 2 gives a boundary of 2^255. Search deterministically
		// until the candidate final hash is below it.
		if final[0]&0x80 == 0 {
			header.Nonce = types.EncodeNonce(nonce)
			header.MixDigest = common.Hash(mix)
			return header
		}
	}
}

func TestVerifyHeaderC1Candidate(t *testing.T) {
	header := sealedCandidateHeader(t)
	valid, err := VerifyHeaderC1Candidate(header)
	if err != nil || !valid {
		t.Fatalf("expected valid header, valid=%t err=%v", valid, err)
	}

	tamperedMix := types.CopyHeader(header)
	tamperedMix.MixDigest[0]++
	valid, err = VerifyHeaderC1Candidate(tamperedMix)
	if err != nil || valid {
		t.Fatalf("tampered mix: valid=%t err=%v", valid, err)
	}

	wrongDifficulty := types.CopyHeader(header)
	wrongDifficulty.Difficulty = big.NewInt(3)
	valid, err = VerifyHeaderC1Candidate(wrongDifficulty)
	if err != nil || valid {
		t.Fatalf("wrong difficulty: valid=%t err=%v", valid, err)
	}
}

func TestVerifyHeaderC1CandidateRejectsUnsupportedInputs(t *testing.T) {
	if _, err := VerifyHeaderC1Candidate(nil); err != ErrNilHeader {
		t.Fatalf("nil header: have %v want %v", err, ErrNilHeader)
	}
	difficultyOne := candidateHeader()
	difficultyOne.Difficulty = big.NewInt(1)
	if _, err := VerifyHeaderC1Candidate(difficultyOne); err != ErrDifficultyBelowMinimum {
		t.Fatalf("difficulty one: have %v want %v", err, ErrDifficultyBelowMinimum)
	}
	largeHeight := candidateHeader()
	largeHeight.Number = new(big.Int).SetUint64(maxC1BlockNumber + 1)
	if _, err := VerifyHeaderC1Candidate(largeHeight); err != ErrUnsupportedBlockNumber {
		t.Fatalf("large height: have %v want %v", err, ErrUnsupportedBlockNumber)
	}
}
