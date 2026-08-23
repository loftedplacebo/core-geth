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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/kawpow"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

var ErrMiningUnavailable = errors.New("KawPoW development engine has no mining implementation")

// DevelopmentEngine delegates only non-seal structural checks to Ethash. Its
// KawPoW seal check is isolated and the engine is not selected by any chain.
type DevelopmentEngine struct{ structural consensus.Engine }

var _ consensus.PoW = (*DevelopmentEngine)(nil)

func New(config ethash.Config) *DevelopmentEngine {
	return &DevelopmentEngine{structural: ethash.New(config, nil, false)}
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
func (e *DevelopmentEngine) Seal(consensus.ChainHeaderReader, *types.Block, chan<- *types.Block, <-chan struct{}) error {
	return ErrMiningUnavailable
}
func (e *DevelopmentEngine) SealHash(h *types.Header) common.Hash { return kawpow.SealHash(h) }
func (e *DevelopmentEngine) CalcDifficulty(c consensus.ChainHeaderReader, t uint64, p *types.Header) *big.Int {
	return e.structural.CalcDifficulty(c, t, p)
}
func (e *DevelopmentEngine) APIs(c consensus.ChainHeaderReader) []rpc.API {
	return e.structural.APIs(c)
}
func (e *DevelopmentEngine) Close() error      { return e.structural.Close() }
func (e *DevelopmentEngine) Hashrate() float64 { return 0 }
