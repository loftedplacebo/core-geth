// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//go:build cgo
// +build cgo

package eth

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/kawpowengine"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

const developmentTemplateLimit = 64

var errDevelopmentTemplateUnavailable = errors.New("KawPoW development block template is unavailable")

type developmentTemplateStore struct {
	mu     sync.Mutex
	engine consensus.Engine
	chain  interface {
		CurrentHeader() *types.Header
		InsertChain(types.Blocks) (int, error)
	}
	events interface {
		Post(interface{}) error
	}
	templates interface {
		PendingBlock() *types.Block
	}
	blocks map[common.Hash]*types.Block
	order  []common.Hash
}

func newKawpowDevelopmentEngine(config ethash.Config) (consensus.Engine, error) {
	return beacon.New(kawpowengine.NewDevelopment(config)), nil
}

func (s *Ethereum) configureKawpowDevelopment() error {
	if !s.config.KawpowDevelopment {
		return nil
	}
	beaconEngine, ok := s.engine.(*beacon.Beacon)
	if !ok {
		return errors.New("KawPoW development engine is not beacon-wrapped")
	}
	templates, ok := beaconEngine.InnerEngine().(interface{ PendingBlock() *types.Block })
	if !ok {
		return errors.New("KawPoW development engine has no finalized template provider")
	}
	store := &developmentTemplateStore{
		engine:    s.engine,
		chain:     s.blockchain,
		events:    s.eventMux,
		templates: templates,
		blocks:    make(map[common.Hash]*types.Block),
	}
	registry, err := kawpowengine.NewWorkRegistry(
		time.Minute,
		developmentTemplateLimit,
		2,
		func(parent common.Hash) bool {
			current := s.blockchain.CurrentHeader()
			return current != nil && current.Hash() == parent
		},
		func(header *types.Header) error {
			return s.engine.VerifyHeader(s.blockchain, header, true)
		},
	)
	if err != nil {
		return fmt.Errorf("create KawPoW development work registry: %w", err)
	}
	service, err := kawpowengine.NewDevelopmentWorkService(registry, store.nextHeader, store.acceptHeader)
	if err != nil {
		return fmt.Errorf("create KawPoW development work service: %w", err)
	}
	s.developmentAPIs = []rpc.API{{Namespace: "aichain", Service: service, Public: false}}
	return nil
}

func (s *developmentTemplateStore) nextHeader() (*types.Header, error) {
	block := s.templates.PendingBlock()
	if block == nil {
		return nil, errDevelopmentTemplateUnavailable
	}
	header := types.CopyHeader(block.Header())
	header.Nonce = types.BlockNonce{}
	header.MixDigest = common.Hash{}
	sealHash := s.engine.SealHash(header)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.blocks[sealHash]; !exists {
		for len(s.blocks) >= developmentTemplateLimit {
			oldest := s.order[0]
			s.order = s.order[1:]
			delete(s.blocks, oldest)
		}
		s.blocks[sealHash] = block
		s.order = append(s.order, sealHash)
	}
	return header, nil
}

func (s *developmentTemplateStore) acceptHeader(header *types.Header) (common.Hash, error) {
	sealHash := s.engine.SealHash(header)
	s.mu.Lock()
	template := s.blocks[sealHash]
	s.mu.Unlock()
	if template == nil {
		return common.Hash{}, errDevelopmentTemplateUnavailable
	}
	current := s.chain.CurrentHeader()
	if current == nil || header.ParentHash != current.Hash() {
		return common.Hash{}, kawpowengine.ErrStaleWork
	}
	sealed := template.WithSeal(header)
	if _, err := s.chain.InsertChain(types.Blocks{sealed}); err != nil {
		return common.Hash{}, err
	}
	// External sealing bypasses miner.worker.resultLoop, so reproduce its
	// standard event after canonical insertion. The handler subscribes to this
	// event and propagates then announces the block to connected peers.
	_ = s.events.Post(core.NewMinedBlockEvent{Block: sealed})
	s.mu.Lock()
	delete(s.blocks, sealHash)
	s.mu.Unlock()
	return sealed.Hash(), nil
}
