package kawpow

import "testing"

// BenchmarkVerifyHeaderC1CandidateCachedEpoch measures repeated CPU
// verification after the candidate's one-epoch native cache has been warmed.
// It is not a mining, block-import, or network-throughput benchmark.
func BenchmarkVerifyHeaderC1CandidateCachedEpoch(b *testing.B) {
	header := sealedCandidateHeader(b)
	if valid, err := VerifyHeaderC1Candidate(header); err != nil || !valid {
		b.Fatalf("warm-up verification failed: valid=%t err=%v", valid, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid, err := VerifyHeaderC1Candidate(header)
		if err != nil || !valid {
			b.Fatalf("verification failed: valid=%t err=%v", valid, err)
		}
	}
}

// BenchmarkVerifyHeaderC1CandidateParallel reports the effect of the current
// conservative native cache lock under concurrent verifier calls. It is a
// stress indicator, not a claim about a final production cache design.
func BenchmarkVerifyHeaderC1CandidateParallel(b *testing.B) {
	header := sealedCandidateHeader(b)
	if valid, err := VerifyHeaderC1Candidate(header); err != nil || !valid {
		b.Fatalf("warm-up verification failed: valid=%t err=%v", valid, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			valid, err := VerifyHeaderC1Candidate(header)
			if err != nil || !valid {
				b.Fatalf("verification failed: valid=%t err=%v", valid, err)
			}
		}
	})
}
