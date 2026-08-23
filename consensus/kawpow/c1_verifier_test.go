package kawpow

import (
	"encoding/hex"
	"strconv"
	"testing"
)

func mustC1Hash(t *testing.T, value string) [32]byte {
	t.Helper()
	var out [32]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != len(out) {
		t.Fatalf("invalid fixture %q: %v", value, err)
	}
	copy(out[:], decoded)
	return out
}

func TestC1VerifierPinnedProgPowVectorZero(t *testing.T) {
	var header [32]byte
	wantMix := mustC1Hash(t, "6e97b47b134fda0c7888802988e1a373affeb28bcd813b6e9a0fc669c935d03a")
	wantFinal := mustC1Hash(t, "e601a7257a70dc48fccc97a7330d704d776047623b92883d77111fb36870f3d1")
	mix, final, err := C1Hash(0, header, 0)
	if err != nil {
		t.Fatal(err)
	}
	if mix != wantMix || final != wantFinal {
		t.Fatalf("unexpected C1 output: mix=%x final=%x", mix, final)
	}
	valid, err := VerifyC1Seal(C1Seal{BlockNumber: 0, HeaderHash: header, MixHash: mix, Boundary: final})
	if err != nil || !valid {
		t.Fatalf("expected valid seal, valid=%t err=%v", valid, err)
	}
	mix[0]++
	valid, err = VerifyC1Seal(C1Seal{BlockNumber: 0, HeaderHash: header, MixHash: mix, Boundary: final})
	if err != nil || valid {
		t.Fatalf("tampered mix: valid=%t err=%v", valid, err)
	}
}

func TestC1VerifierRejectsOutOfRangeBlockNumber(t *testing.T) {
	// On 32-bit Go, the type itself cannot represent a value above the
	// reference API's signed-32-bit maximum.
	if strconv.IntSize <= 32 {
		t.Skip("int cannot represent an out-of-range C1 block number")
	}
	outOfRange := int(maxC1BlockNumber + 1)
	if _, _, err := C1Hash(outOfRange, [32]byte{}, 0); err != ErrUnsupportedBlockNumber {
		t.Fatalf("hash: have %v want %v", err, ErrUnsupportedBlockNumber)
	}
	if _, err := VerifyC1Seal(C1Seal{BlockNumber: outOfRange}); err != ErrUnsupportedBlockNumber {
		t.Fatalf("verify: have %v want %v", err, ErrUnsupportedBlockNumber)
	}
}
