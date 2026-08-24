// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//go:build cgo
// +build cgo

//
// The AIChain Core-Geth fork is free software: you can redistribute it and/or
// modify it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the License,
// or (at your option) any later version.

// Package kawpowengine contains the disabled-by-default Phase 2A KawPoW engine.
package kawpowengine

import (
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/kawpow"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	ErrMiningUnavailable                = errors.New("KawPoW development engine has no mining implementation")
	ErrInvalidDevelopmentSealingRequest = errors.New("invalid KawPoW development sealing request")
)

// DevelopmentEngine delegates non-seal structural checks to Ethash and uses
// the isolated KawPoW verifier for seals. It is selected only by explicit G2
// development mode; normal chain configurations retain their existing engine.
type DevelopmentEngine struct {
	structural         consensus.Engine
	developmentSealing atomic.Bool
	latestTemplate     atomic.Pointer[types.Block]
}

var _ consensus.PoW = (*DevelopmentEngine)(nil)

func New(config ethash.Config) *DevelopmentEngine {
	return &DevelopmentEngine{structural: ethash.New(config, nil, false)}
}

// NewDevelopment creates the explicitly enabled G2 engine. It still performs
// no CPU mining: Seal only retains Core-Geth's normal pending-task lifecycle
// while the local development RPC supplies an externally discovered seal.
func NewDevelopment(config ethash.Config) *DevelopmentEngine {
	engine := New(config)
	engine.developmentSealing.Store(true)
	return engine
}

func (e *DevelopmentEngine) Author(h *types.Header) (common.Address, error) {
	return e.structural.Author(h)
}

func (e *DevelopmentEngine) VerifyHeader(c consensus.ChainHeaderReader, h *types.Header, seal bool) error {
	if err := e.structural.VerifyHeader(c, h, false); err != nil {
		return err
	}
	if seal {
		return e.VerifySeal(h)
	}
	return nil
}

func (e *DevelopmentEngine) VerifySeal(h *types.Header) error {
	ok, err := kawpow.VerifyHeaderC1Candidate(h)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("invalid KawPoW proof-of-work")
	}
	return nil
}

func (e *DevelopmentEngine) VerifyHeaders(c consensus.ChainHeaderReader, hs []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort, results := make(chan struct{}), make(chan error, len(hs))
	go func() {
		defer close(results)
		for i, h := range hs {
			select {
			case <-abort:
				return
			case results <- e.VerifyHeader(c, h, i < len(seals) && seals[i]):
			}
		}
	}()
	return abort, results
}

func (e *DevelopmentEngine) VerifyUncles(c consensus.ChainReader, b *types.Block) error {
	return e.structural.VerifyUncles(c, b)
}
func (e *DevelopmentEngine) Prepare(c consensus.ChainHeaderReader, h *types.Header) error {
	return e.structural.Prepare(c, h)
}
func (e *DevelopmentEngine) Finalize(c consensus.ChainHeaderReader, h *types.Header, s *state.StateDB, txs []*types.Transaction, u []*types.Header, w []*types.Withdrawal) {
	e.structural.Finalize(c, h, s, txs, u, w)
}
func (e *DevelopmentEngine) FinalizeAndAssemble(c consensus.ChainHeaderReader, h *types.Header, s *state.StateDB, txs []*types.Transaction, u []*types.Header, r []*types.Receipt, w []*types.Withdrawal) (*types.Block, error) {
	return e.structural.FinalizeAndAssemble(c, h, s, txs, u, r, w)
}
func (e *DevelopmentEngine) Seal(_ consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	if !e.developmentSealing.Load() {
		return ErrMiningUnavailable
	}
	if block == nil || results == nil || stop == nil {
		return ErrInvalidDevelopmentSealingRequest
	}
	e.latestTemplate.Store(block)
	return nil
}

// PendingBlock returns the latest fully finalized block submitted to Seal.
// The block is immutable; only its copied header receives an external seal.
func (e *DevelopmentEngine) PendingBlock() *types.Block           { return e.latestTemplate.Load() }
func (e *DevelopmentEngine) SealHash(h *types.Header) common.Hash { return kawpow.SealHash(h) }
func (e *DevelopmentEngine) CalcDifficulty(c consensus.ChainHeaderReader, t uint64, p *types.Header) *big.Int {
	return DevelopmentCalcDifficulty(c.Config(), t, p)
}

// DevelopmentCalcDifficulty deliberately preserves the current Core-Geth
// difficulty calculation for the disabled engine spike. It is not a selected
// KawPoW production retarget rule.
func DevelopmentCalcDifficulty(config ctypes.ChainConfigurator, time uint64, parent *types.Header) *big.Int {
	return ethash.CalcDifficulty(config, time, parent)
}
func (e *DevelopmentEngine) APIs(c consensus.ChainHeaderReader) []rpc.API {
	return e.structural.APIs(c)
}
func (e *DevelopmentEngine) Close() error      { return e.structural.Close() }
func (e *DevelopmentEngine) Hashrate() float64 { return 0 }
