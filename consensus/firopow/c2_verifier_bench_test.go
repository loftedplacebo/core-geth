package firopow

import "testing"

func c2OfficialVectorSeal(b *testing.B) C2Seal {
	b.Helper()
	header := mustHash(b, "2d794e900dcad779e658de9078d9a88eee87d75f7b09a8fdd270d3a8e76650c7")
	boundary := mustHash(b, "0001869e7a058e2aaf266cd2f166fb85c98d651e60eadbbe72bb0a36f8802805")
	mix := mustHash(b, "cfab3766331d6c4e6913e6688a71e4c26b7f36c1581cdbec0f5b19db8956eb50")
	return C2Seal{BlockNumber: 1, HeaderHash: header, MixHash: mix, Nonce: 0x85f22c9b3cd2f123, Boundary: boundary}
}

// BenchmarkVerifyC2OfficialVector measures the warmed CPU verifier only. It
// intentionally excludes epoch construction, header mapping, block import,
// mining, and network work.
func BenchmarkVerifyC2OfficialVector(b *testing.B) {
	seal := c2OfficialVectorSeal(b)
	if valid, err := VerifyC2Seal(seal); err != nil || !valid {
		b.Fatalf("warmup failed: valid=%t err=%v", valid, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid, err := VerifyC2Seal(seal)
		if err != nil || !valid {
			b.Fatalf("verify failed: valid=%t err=%v", valid, err)
		}
	}
}

// BenchmarkVerifyC2OfficialVectorParallel exercises the same deliberately
// conservative single-epoch cache under concurrent callers. It is not a TPS
// estimate or a production cache design.
func BenchmarkVerifyC2OfficialVectorParallel(b *testing.B) {
	seal := c2OfficialVectorSeal(b)
	if valid, err := VerifyC2Seal(seal); err != nil || !valid {
		b.Fatalf("warmup failed: valid=%t err=%v", valid, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			valid, err := VerifyC2Seal(seal)
			if err != nil || !valid {
				b.Fatalf("verify failed: valid=%t err=%v", valid, err)
			}
		}
	})
}
