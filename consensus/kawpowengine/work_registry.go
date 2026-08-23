// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//go:build cgo
// +build cgo

package kawpowengine

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/kawpow"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const DevelopmentWorkVersion = "0.1.0-dev"

var (
	ErrUnknownWork       = errors.New("unknown KawPoW work")
	ErrStaleWork         = errors.New("stale KawPoW work")
	ErrDuplicateWork     = errors.New("duplicate KawPoW work")
	ErrInvalidWorkConfig = errors.New("invalid KawPoW work registry configuration")
)

// DevelopmentWork is an immutable node-issued mining template. It contains no
// caller-controlled consensus fields.
type DevelopmentWork struct {
	Version    string
	ID         common.Hash
	HeaderHash common.Hash
	ParentHash common.Hash
	Height     uint64
	Target     [32]byte
	ExpiresAt  uint64
}

type workRecord struct {
	work     DevelopmentWork
	header   *types.Header
	expires  time.Time
	accepted bool
}

// WorkRegistry owns the bounded development mining-work state machine. It is
// deliberately independent of RPC transport and block import.
type WorkRegistry struct {
	mu            sync.Mutex
	now           func() time.Time
	isCanonical   func(common.Hash) bool
	verify        func(*types.Header) error
	ttl           time.Duration
	maxActive     int
	verifySlots   chan struct{}
	records       map[common.Hash]*workRecord
	issuanceOrder []common.Hash
}

func NewWorkRegistry(
	ttl time.Duration,
	maxActive int,
	maxConcurrentVerification int,
	isCanonical func(common.Hash) bool,
	verify func(*types.Header) error,
) (*WorkRegistry, error) {
	if ttl < 10*time.Second || ttl > 300*time.Second || maxActive < 1 || maxActive > 64 ||
		maxConcurrentVerification < 1 || isCanonical == nil || verify == nil {
		return nil, ErrInvalidWorkConfig
	}
	return &WorkRegistry{
		now:         time.Now,
		isCanonical: isCanonical,
		verify:      verify,
		ttl:         ttl,
		maxActive:   maxActive,
		verifySlots: make(chan struct{}, maxConcurrentVerification),
		records:     make(map[common.Hash]*workRecord),
	}, nil
}

// Issue validates and copies a node-created header before exposing its mining
// fields. Reissuing the same template refreshes neither identity nor expiry.
func (r *WorkRegistry) Issue(header *types.Header) (DevelopmentWork, error) {
	input, err := kawpow.HeaderToVerificationInput(header)
	if err != nil {
		return DevelopmentWork{}, err
	}
	if input.Height > kawpow.MaxC1BlockNumber {
		return DevelopmentWork{}, kawpow.ErrUnsupportedBlockNumber
	}
	if input.Target.BitLen() > 256 {
		return DevelopmentWork{}, kawpow.ErrTargetOutOfRange
	}
	if !r.isCanonical(header.ParentHash) {
		return DevelopmentWork{}, ErrStaleWork
	}

	now := r.now()
	var target [32]byte
	input.Target.FillBytes(target[:])
	id := developmentWorkID(input.SealHash, input.Height, target)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)
	if existing := r.records[id]; existing != nil {
		return existing.work, nil
	}
	for len(r.records) >= r.maxActive {
		r.evictOldestLocked()
	}
	expires := now.Add(r.ttl)
	work := DevelopmentWork{
		Version:    DevelopmentWorkVersion,
		ID:         id,
		HeaderHash: input.SealHash,
		ParentHash: header.ParentHash,
		Height:     input.Height,
		Target:     target,
		ExpiresAt:  uint64(expires.Unix()),
	}
	r.records[id] = &workRecord{work: work, header: types.CopyHeader(header), expires: expires}
	r.issuanceOrder = append(r.issuanceOrder, id)
	return work, nil
}

// Submit reconstructs the node-owned header, verifies the supplied seal, and
// permits at most one accepted result for a work identifier.
func (r *WorkRegistry) Submit(id common.Hash, nonce types.BlockNonce, mixDigest common.Hash) (*types.Header, error) {
	r.verifySlots <- struct{}{}
	defer func() { <-r.verifySlots }()

	r.mu.Lock()
	record := r.records[id]
	if record == nil {
		r.mu.Unlock()
		return nil, ErrUnknownWork
	}
	if record.accepted {
		r.mu.Unlock()
		return nil, ErrDuplicateWork
	}
	if !r.now().Before(record.expires) || !r.isCanonical(record.work.ParentHash) {
		delete(r.records, id)
		r.mu.Unlock()
		return nil, ErrStaleWork
	}
	header := types.CopyHeader(record.header)
	r.mu.Unlock()

	header.Nonce = nonce
	header.MixDigest = mixDigest
	if err := r.verify(header); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	record = r.records[id]
	if record == nil || !r.now().Before(record.expires) || !r.isCanonical(record.work.ParentHash) {
		delete(r.records, id)
		return nil, ErrStaleWork
	}
	if record.accepted {
		return nil, ErrDuplicateWork
	}
	record.accepted = true
	return header, nil
}

func (r *WorkRegistry) pruneLocked(now time.Time) {
	for id, record := range r.records {
		if !now.Before(record.expires) || !r.isCanonical(record.work.ParentHash) {
			delete(r.records, id)
		}
	}
	for len(r.issuanceOrder) > 0 {
		if _, ok := r.records[r.issuanceOrder[0]]; ok {
			break
		}
		r.issuanceOrder = r.issuanceOrder[1:]
	}
}

func (r *WorkRegistry) evictOldestLocked() {
	for len(r.issuanceOrder) > 0 {
		id := r.issuanceOrder[0]
		r.issuanceOrder = r.issuanceOrder[1:]
		if _, ok := r.records[id]; ok {
			delete(r.records, id)
			return
		}
	}
}

func developmentWorkID(headerHash common.Hash, height uint64, target [32]byte) common.Hash {
	var encodedHeight [8]byte
	binary.BigEndian.PutUint64(encodedHeight[:], height)
	return crypto.Keccak256Hash(
		[]byte("aichain-kawpow-work-v1\x00"),
		headerHash[:],
		encodedHeight[:],
		target[:],
	)
}
