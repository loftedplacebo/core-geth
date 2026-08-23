// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//
// The AIChain Core-Geth fork is free software: you can redistribute it and/or
// modify it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the License,
// or (at your option) any later version.

// Package kawpow contains the isolated C1 KawPoW candidate boundary.
//
// It intentionally does not implement consensus.Engine, mining, or engine
// selection. Those steps require an explicit protocol decision and a separate
// activation plan. The package fixes only the header fields that a future
// reference verifier must receive.
package kawpow

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	// ErrNilHeader is returned when a verification input cannot be derived.
	ErrNilHeader = errors.New("nil header")
	// ErrInvalidDifficulty is returned when the header has no positive difficulty.
	ErrInvalidDifficulty = errors.New("non-positive difficulty")
)

// VerificationInput is the explicit boundary between a Core-Geth header and
// the external KawPoW reference verifier evaluated in Phase 2A.
//
// SealHash follows the existing Ethash pre-seal RLP encoding. This is a
// candidate mapping, not a decision to retain Ethash's header format in a
// launched network.
type VerificationInput struct {
	SealHash  common.Hash
	Nonce     uint64
	MixDigest common.Hash
	Height    uint64
	Target    *big.Int
}

// HeaderToVerificationInput derives the candidate verifier input from a block
// header. It rejects malformed difficulty values before a native verifier is
// called, so callers have one clear validation boundary.
func HeaderToVerificationInput(header *types.Header) (*VerificationInput, error) {
	if header == nil || header.Number == nil {
		return nil, ErrNilHeader
	}
	if header.Difficulty == nil || header.Difficulty.Sign() <= 0 {
		return nil, ErrInvalidDifficulty
	}
	return &VerificationInput{
		SealHash:  SealHash(header),
		Nonce:     uint64(header.Nonce),
		MixDigest: header.MixDigest,
		Height:    header.Number.Uint64(),
		Target:    new(big.Int).Div(new(big.Int).Lsh(big.NewInt(1), 256), header.Difficulty),
	}, nil
}

// SealHash returns the RLP hash of a header before proof-of-work fields are
// considered. It mirrors the current Ethash encoding solely to make the C1
// candidate mapping testable against Core-Geth's existing implementation.
func SealHash(header *types.Header) (hash common.Hash) {
	hasher := crypto.NewKeccakState()
	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if header.WithdrawalsHash != nil || header.ExcessBlobGas != nil || header.BlobGasUsed != nil || header.ParentBeaconRoot != nil {
		panic("unsupported post-Ethash header field set")
	}
	rlp.Encode(hasher, enc)
	hasher.Read(hash[:])
	return hash
}
