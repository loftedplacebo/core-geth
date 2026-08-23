package firopow

import (
	"encoding/hex"
	"strconv"
	"testing"
)

func mustHash(t *testing.T, value string) [32]byte {
	t.Helper()
	var out [32]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != len(out) {
		t.Fatalf("invalid fixture %q: %v", value, err)
	}
	copy(out[:], decoded)
	return out
}

func TestC2VerifierOfficialFiroPoWVector(t *testing.T) {
	header := mustHash(t, "2d794e900dcad779e658de9078d9a88eee87d75f7b09a8fdd270d3a8e76650c7")
	boundary := mustHash(t, "0001869e7a058e2aaf266cd2f166fb85c98d651e60eadbbe72bb0a36f8802805")
	wantMix := mustHash(t, "cfab3766331d6c4e6913e6688a71e4c26b7f36c1581cdbec0f5b19db8956eb50")
	wantFinal := mustHash(t, "00017c7de1fa499314f9e3dd3537546982073624f7d478592cf28a6d13929f2d")

	mix, final, err := C2Hash(1, header, 0x85f22c9b3cd2f123)
	if err != nil {
		t.Fatal(err)
	}
	if mix != wantMix || final != wantFinal {
		t.Fatalf("unexpected C2 output: mix=%x final=%x", mix, final)
	}
	valid, err := VerifyC2Seal(C2Seal{BlockNumber: 1, HeaderHash: header, MixHash: mix, Nonce: 0x85f22c9b3cd2f123, Boundary: boundary})
	if err != nil || !valid {
		t.Fatalf("expected valid seal, valid=%t err=%v", valid, err)
	}
	mix[0]++
	valid, err = VerifyC2Seal(C2Seal{BlockNumber: 1, HeaderHash: header, MixHash: mix, Nonce: 0x85f22c9b3cd2f123, Boundary: boundary})
	if err != nil || valid {
		t.Fatalf("tampered mix: valid=%t err=%v", valid, err)
	}
}

func TestC2VerifierRejectsUnsupportedBlockNumbers(t *testing.T) {
	if _, _, err := C2Hash(-1, [32]byte{}, 0); err != ErrUnsupportedBlockNumber {
		t.Fatalf("negative hash: have %v want %v", err, ErrUnsupportedBlockNumber)
	}
	if strconv.IntSize <= 32 {
		t.Skip("int cannot represent an out-of-range C2 block number")
	}
	if _, _, err := C2Hash(int(maxC2BlockNumber+1), [32]byte{}, 0); err != ErrUnsupportedBlockNumber {
		t.Fatalf("out-of-range hash: have %v want %v", err, ErrUnsupportedBlockNumber)
	}
}
