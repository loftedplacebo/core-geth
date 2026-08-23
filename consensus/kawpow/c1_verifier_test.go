package kawpow

import (
	"encoding/hex"
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
