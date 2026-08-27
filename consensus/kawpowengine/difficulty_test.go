//go:build cgo
// +build cgo

package kawpowengine

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
	"github.com/ethereum/go-ethereum/params/types/goethereum"
)

type difficultyTestChain struct {
	config  ctypes.ChainConfigurator
	headers map[uint64]*types.Header
	current *types.Header
}

func newDifficultyTestChain(headers ...*types.Header) *difficultyTestChain {
	chain := &difficultyTestChain{
		config:  &goethereum.ChainConfig{},
		headers: make(map[uint64]*types.Header),
	}
	for _, header := range headers {
		chain.headers[header.Number.Uint64()] = header
		if chain.current == nil || header.Number.Cmp(chain.current.Number) > 0 {
			chain.current = header
		}
	}
	return chain
}

func (chain *difficultyTestChain) Config() ctypes.ChainConfigurator { return chain.config }
func (chain *difficultyTestChain) CurrentHeader() *types.Header     { return chain.current }
func (chain *difficultyTestChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	header := chain.headers[number]
	if header != nil && header.Hash() == hash {
		return header
	}
	return nil
}
func (chain *difficultyTestChain) GetHeaderByNumber(number uint64) *types.Header {
	return chain.headers[number]
}
func (chain *difficultyTestChain) GetHeaderByHash(hash common.Hash) *types.Header {
	for _, header := range chain.headers {
		if header.Hash() == hash {
			return header
		}
	}
	return nil
}
func (chain *difficultyTestChain) GetTd(hash common.Hash, number uint64) *big.Int {
	if chain.GetHeader(hash, number) == nil {
		return nil
	}
	return big.NewInt(1)
}

func developmentGenesis() *types.Header {
	return &types.Header{
		UncleHash:  types.EmptyUncleHash,
		Number:     big.NewInt(0),
		Time:       1_000,
		Difficulty: big.NewInt(327_680),
		GasLimit:   30_000_000,
	}
}

func developmentChild(parent *types.Header, timestamp uint64) *types.Header {
	return &types.Header{
		ParentHash: parent.Hash(),
		UncleHash:  types.EmptyUncleHash,
		Number:     new(big.Int).Add(parent.Number, big.NewInt(1)),
		Time:       timestamp,
		Difficulty: big.NewInt(1),
		GasLimit:   parent.GasLimit,
	}
}

func TestDevelopmentASERTConfigurationIsClosed(t *testing.T) {
	for _, target := range []uint64{5, 10, 15} {
		config, err := NewDevelopmentASERTConfig(target)
		if err != nil {
			t.Fatalf("target %d: %v", target, err)
		}
		if err := config.Validate(); err != nil {
			t.Fatalf("target %d validation: %v", target, err)
		}
	}
	for _, target := range []uint64{0, 1, 9, 11, 60} {
		if _, err := NewDevelopmentASERTConfig(target); err == nil {
			t.Fatalf("unsupported target %d accepted", target)
		}
	}
	config, _ := NewDevelopmentASERTConfig(10)
	config.HalfLifeSeconds++
	if err := config.Validate(); err == nil {
		t.Fatal("non-selected half-life accepted")
	}
	config, _ = NewDevelopmentASERTConfig(10)
	config.ActivationBlock++
	if err := config.Validate(); err == nil {
		t.Fatal("non-genesis activation accepted")
	}
}

func TestCommittedDevelopmentASERTVectors(t *testing.T) {
	var vectorSet struct {
		TargetSeconds uint64
		HalfLife      uint64
		AnchorHeight  string
		AnchorTime    uint64
		AnchorTarget  string
		Vectors       []struct {
			Name               string
			CandidateHeight    string
			CandidateTimestamp uint64
			ExpectedTarget     string
		}
	}
	payload, err := os.ReadFile("testdata/asert-v1-1800-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload, &vectorSet); err != nil {
		t.Fatal(err)
	}
	parse := func(value string) *big.Int {
		parsed, ok := new(big.Int).SetString(value, 10)
		if !ok {
			t.Fatalf("invalid integer %q", value)
		}
		return parsed
	}
	config := &DevelopmentASERTConfig{
		TargetSeconds:   vectorSet.TargetSeconds,
		HalfLifeSeconds: vectorSet.HalfLife,
		ActivationBlock: DevelopmentASERTActivationBlock,
	}
	for _, vector := range vectorSet.Vectors {
		target, err := calculateASERTTarget(
			config,
			parse(vectorSet.AnchorTarget),
			parse(vectorSet.AnchorHeight),
			vectorSet.AnchorTime,
			parse(vector.CandidateHeight),
			vector.CandidateTimestamp,
		)
		if err != nil {
			t.Fatalf("%s: %v", vector.Name, err)
		}
		if expected := parse(vector.ExpectedTarget); target.Cmp(expected) != 0 {
			t.Fatalf("%s: target %s, want %s", vector.Name, target, expected)
		}
	}
}

func TestDevelopmentASERTPrepareSingleAndBatchVerification(t *testing.T) {
	genesis := developmentGenesis()
	chain := newDifficultyTestChain(genesis)
	engine, err := NewDevelopmentASERT(ethash.Config{PowMode: ethash.ModeFake}, 10)
	if err != nil {
		t.Fatal(err)
	}
	child := developmentChild(genesis, genesis.Time+10)
	if err := engine.Prepare(chain, child); err != nil {
		t.Fatal(err)
	}
	if child.Difficulty.Cmp(genesis.Difficulty) != 0 {
		t.Fatalf("on-schedule difficulty %s, want %s", child.Difficulty, genesis.Difficulty)
	}
	if err := engine.VerifyHeader(chain, child, false); err != nil {
		t.Fatalf("prepared header rejected: %v", err)
	}
	tampered := types.CopyHeader(child)
	tampered.Difficulty.Add(tampered.Difficulty, big.NewInt(1))
	if err := engine.VerifyHeader(chain, tampered, false); err == nil {
		t.Fatal("tampered ASERT difficulty accepted")
	}
	second := developmentChild(child, child.Time+10)
	difficulty, err := engine.asert.Calculate(chain, second.Time, child)
	if err != nil {
		t.Fatal(err)
	}
	second.Difficulty = difficulty
	_, results := engine.VerifyHeaders(chain, []*types.Header{child, second}, []bool{false, false})
	for index := 0; index < 2; index++ {
		if err := <-results; err != nil {
			t.Fatalf("batch header %d rejected: %v", index, err)
		}
	}
}

func TestDevelopmentASERTBoundsAnchorAndReorg(t *testing.T) {
	genesis := developmentGenesis()
	chain := newDifficultyTestChain(genesis)
	config, _ := NewDevelopmentASERTConfig(10)
	if _, err := config.Calculate(chain, genesis.Time, genesis); err == nil {
		t.Fatal("non-increasing timestamp accepted")
	}
	late, err := config.Calculate(chain, genesis.Time+1_000_000, genesis)
	if err != nil {
		t.Fatal(err)
	}
	if late.Cmp(asertTwo) != 0 {
		t.Fatalf("late difficulty %s, want minimum 2", late)
	}
	earlyParent := &types.Header{
		Number:     new(big.Int).SetUint64(1_000_000_000),
		Time:       genesis.Time,
		Difficulty: new(big.Int).Set(genesis.Difficulty),
	}
	early, err := config.Calculate(chain, genesis.Time+1, earlyParent)
	if err != nil {
		t.Fatal(err)
	}
	if early.Cmp(asertMaxUint256) != 0 {
		t.Fatalf("early difficulty %s, want maximum uint256", early)
	}
	parentA := &types.Header{Number: big.NewInt(10), Time: 1_090, Difficulty: big.NewInt(400_000)}
	parentB := &types.Header{Number: big.NewInt(10), Time: 1_120, Difficulty: big.NewInt(200_000)}
	a, err := config.Calculate(chain, 1_130, parentA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := config.Calculate(chain, 1_130, parentB)
	if err != nil {
		t.Fatal(err)
	}
	if a.Cmp(b) != 0 {
		t.Fatalf("same anchor/height/time diverged across reorg branches: %s vs %s", a, b)
	}
	missingAnchor := newDifficultyTestChain()
	if _, err := config.Calculate(missingAnchor, 1_130, parentA); err == nil {
		t.Fatal("missing ASERT anchor accepted")
	}
}

func TestDevelopmentASERTRejectsUncles(t *testing.T) {
	engine, err := NewDevelopmentASERT(ethash.Config{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	block := types.NewBlockWithHeader(developmentChild(developmentGenesis(), 1_010)).WithBody(nil, []*types.Header{{Number: big.NewInt(1)}})
	if err := engine.VerifyUncles(nil, block); err != ErrDevelopmentUnclesUnsupported {
		t.Fatalf("uncle error %v, want %v", err, ErrDevelopmentUnclesUnsupported)
	}
}

func FuzzDevelopmentASERTDifficulty(f *testing.F) {
	for _, seed := range []struct {
		height, parentTime, candidateTime uint64
	}{
		{0, 1_000, 1_010},
		{1, 1_010, 1_011},
		{100, 2_000, 2_100},
		{1 << 31, 1_000, 1_000_000},
	} {
		f.Add(seed.height, seed.parentTime, seed.candidateTime)
	}
	f.Fuzz(func(t *testing.T, height, parentTime, candidateTime uint64) {
		if candidateTime <= parentTime || height > 1<<32 {
			t.Skip()
		}
		genesis := developmentGenesis()
		chain := newDifficultyTestChain(genesis)
		config, _ := NewDevelopmentASERTConfig(10)
		parent := &types.Header{
			Number:     new(big.Int).SetUint64(height),
			Time:       parentTime,
			Difficulty: big.NewInt(327_680),
		}
		difficulty, err := config.Calculate(chain, candidateTime, parent)
		if err != nil {
			t.Fatal(err)
		}
		if difficulty.Sign() <= 0 || difficulty.BitLen() > 256 {
			t.Fatalf("out-of-bounds difficulty %s", difficulty)
		}
	})
}

var _ consensus.ChainHeaderReader = (*difficultyTestChain)(nil)
