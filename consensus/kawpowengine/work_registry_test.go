//go:build cgo
// +build cgo

package kawpowengine

import (
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/types"
)

func workHeader(parent byte, number int64) *types.Header {
	return &types.Header{
		ParentHash: common.BytesToHash([]byte{parent}),
		UncleHash:  types.EmptyUncleHash, TxHash: types.EmptyTxsHash,
		ReceiptHash: types.EmptyReceiptsHash, Difficulty: big.NewInt(2),
		Number: big.NewInt(number), GasLimit: 30_000_000, Time: 1_700_000_000,
	}
}

func testRegistry(t *testing.T, canonical *atomic.Bool, verify func(*types.Header) error) *WorkRegistry {
	t.Helper()
	r, err := NewWorkRegistry(time.Minute, 2, 2, func(common.Hash) bool { return canonical.Load() }, verify)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestWorkRegistryIssueIsDeterministicAndBounded(t *testing.T) {
	var canonical atomic.Bool
	canonical.Store(true)
	r := testRegistry(t, &canonical, func(*types.Header) error { return nil })
	now := time.Unix(1_700_000_000, 0)
	r.now = func() time.Time { return now }

	first, err := r.Issue(workHeader(1, 42))
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.Issue(workHeader(1, 42))
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Fatalf("reissued work changed: %#v != %#v", first, again)
	}
	if first.Version != DevelopmentWorkVersion || first.Height != 42 || first.ExpiresAt != uint64(now.Add(time.Minute).Unix()) {
		t.Fatalf("unexpected work: %#v", first)
	}
	if first.HeaderHash == (common.Hash{}) || first.ID == (common.Hash{}) {
		t.Fatal("work is missing identifiers")
	}

	second, err := r.Issue(workHeader(1, 43))
	if err != nil {
		t.Fatal(err)
	}
	third, err := r.Issue(workHeader(1, 44))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Submit(first.ID, types.BlockNonce{}, common.Hash{}); !errors.Is(err, ErrUnknownWork) {
		t.Fatalf("evicted work error = %v", err)
	}
	if second.ID == third.ID {
		t.Fatal("different templates share a work ID")
	}
}

func TestWorkRegistryAcceptsOnceAndPreservesTemplate(t *testing.T) {
	var canonical atomic.Bool
	canonical.Store(true)
	var verified atomic.Int32
	r := testRegistry(t, &canonical, func(*types.Header) error { verified.Add(1); return nil })
	original := workHeader(1, 42)
	work, err := r.Issue(original)
	if err != nil {
		t.Fatal(err)
	}
	nonce := types.EncodeNonce(7)
	mix := common.HexToHash("0x1234")
	sealed, err := r.Submit(work.ID, nonce, mix)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Nonce != nonce || sealed.MixDigest != mix || sealed.ParentHash != original.ParentHash || sealed.Number.Cmp(original.Number) != 0 {
		t.Fatalf("sealed header changed node-owned fields: %#v", sealed)
	}
	if original.Nonce != (types.BlockNonce{}) || original.MixDigest != (common.Hash{}) {
		t.Fatal("issuing or submitting mutated caller header")
	}
	if _, err := r.Submit(work.ID, nonce, mix); !errors.Is(err, ErrDuplicateWork) {
		t.Fatalf("duplicate error = %v", err)
	}
	if verified.Load() != 1 {
		t.Fatalf("verify calls = %d, want 1", verified.Load())
	}
}

func TestWorkRegistryAcceptsRealC1Seal(t *testing.T) {
	var canonical atomic.Bool
	canonical.Store(true)
	engine := New(ethash.Config{})
	r := testRegistry(t, &canonical, engine.VerifySeal)
	sealed := sealedEngineHeader(t)
	template := types.CopyHeader(sealed)
	template.Nonce = types.BlockNonce{}
	template.MixDigest = common.Hash{}
	work, err := r.Issue(template)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := r.Submit(work.ID, sealed.Nonce, sealed.MixDigest)
	if err != nil {
		t.Fatalf("real C1 seal rejected: %v", err)
	}
	if accepted.Hash() != sealed.Hash() {
		t.Fatalf("accepted block hash = %s, want %s", accepted.Hash(), sealed.Hash())
	}
}

func TestWorkRegistryRejectsInvalidStaleAndExpired(t *testing.T) {
	var canonical atomic.Bool
	canonical.Store(true)
	invalid := errors.New("invalid seal")
	r := testRegistry(t, &canonical, func(*types.Header) error { return invalid })
	now := time.Unix(1_700_000_000, 0)
	r.now = func() time.Time { return now }
	work, err := r.Issue(workHeader(1, 42))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Submit(work.ID, types.BlockNonce{}, common.Hash{}); !errors.Is(err, invalid) {
		t.Fatalf("invalid-seal error = %v", err)
	}
	canonical.Store(false)
	if _, err := r.Submit(work.ID, types.BlockNonce{}, common.Hash{}); !errors.Is(err, ErrStaleWork) {
		t.Fatalf("head-change error = %v", err)
	}
	canonical.Store(true)
	work, err = r.Issue(workHeader(2, 43))
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if _, err := r.Submit(work.ID, types.BlockNonce{}, common.Hash{}); !errors.Is(err, ErrStaleWork) {
		t.Fatalf("expiry error = %v", err)
	}
}

func TestWorkRegistryConcurrentSubmissionAcceptsOnce(t *testing.T) {
	var canonical atomic.Bool
	canonical.Store(true)
	r := testRegistry(t, &canonical, func(*types.Header) error { return nil })
	work, err := r.Issue(workHeader(1, 42))
	if err != nil {
		t.Fatal(err)
	}

	const workers = 16
	var wait sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func(n uint64) {
			defer wait.Done()
			if _, err := r.Submit(work.ID, types.EncodeNonce(n), common.BytesToHash([]byte{byte(n)})); err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrDuplicateWork) {
				t.Errorf("submit error = %v", err)
			}
		}(uint64(i))
	}
	wait.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted = %d, want 1", accepted.Load())
	}
}
