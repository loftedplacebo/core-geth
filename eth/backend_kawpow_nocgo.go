// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//go:build !cgo
// +build !cgo

package eth

import (
	"errors"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
)

var errKawpowDevelopmentRequiresCGO = errors.New("KawPoW development mode requires a CGO-enabled build")

func newKawpowDevelopmentEngine(ethash.Config, uint64) (consensus.Engine, error) {
	return nil, errKawpowDevelopmentRequiresCGO
}

func (s *Ethereum) configureKawpowDevelopment() error {
	if s.config.KawpowDevelopment {
		return errKawpowDevelopmentRequiresCGO
	}
	return nil
}
