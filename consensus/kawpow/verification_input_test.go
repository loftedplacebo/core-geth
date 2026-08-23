package kawpow

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestHeaderToVerificationInputMatchesEthashSealHash(t *testing.T) {
	header := &types.Header{
		ParentHash:  common.HexToHash("0x01"),
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    common.HexToAddress("0x1234"),
		Root:        common.HexToHash("0x02"),
		TxHash:      types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash,
		Difficulty:  big.NewInt(1000),
		Number:      big.NewInt(42),
		GasLimit:    30_000_000,
		Time:        1_700_000_000,
		Extra:       []byte("aichain-c1"),
		Nonce:       types.EncodeNonce(7),
		MixDigest:   common.HexToHash("0x03"),
	}
	input, err := HeaderToVerificationInput(header)
	if err != nil {
		t.Fatal(err)
	}
	if want := ethash.NewTester(nil, false).SealHash(header); input.SealHash != want {
		t.Fatalf("seal hash mismatch: have %x want %x", input.SealHash, want)
	}
	if input.Nonce != 7 || input.Height != 42 || input.MixDigest != header.MixDigest {
		t.Fatalf("header fields were not preserved: %#v", input)
	}
	wantTarget := new(big.Int).Div(new(big.Int).Lsh(big.NewInt(1), 256), header.Difficulty)
	if input.Target.Cmp(wantTarget) != 0 {
		t.Fatalf("target mismatch: have %x want %x", input.Target, wantTarget)
	}
}

func TestHeaderToVerificationInputRejectsInvalidDifficulty(t *testing.T) {
	for _, difficulty := range []*big.Int{nil, big.NewInt(0), big.NewInt(-1)} {
		_, err := HeaderToVerificationInput(&types.Header{Number: big.NewInt(1), Difficulty: difficulty})
		if err != ErrInvalidDifficulty {
			t.Fatalf("difficulty %v: have %v want %v", difficulty, err, ErrInvalidDifficulty)
		}
	}
}
