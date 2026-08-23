// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//go:build cgo
// +build cgo

package kawpowengine

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const MaxDevelopmentSubmissionBytes = 4 * 1024

type DevelopmentSubmitResponse struct {
	Accepted  bool   `json:"accepted"`
	Status    string `json:"status"`
	BlockHash string `json:"blockHash,omitempty"`
}

// DevelopmentWorkService implements the proposed method behavior without
// registering an RPC namespace. Registration and transport exposure remain a
// separate gate.
type DevelopmentWorkService struct {
	registry     *WorkRegistry
	nextTemplate func() (*types.Header, error)
	accept       func(*types.Header) (common.Hash, error)
}

func NewDevelopmentWorkService(
	registry *WorkRegistry,
	nextTemplate func() (*types.Header, error),
	accept func(*types.Header) (common.Hash, error),
) (*DevelopmentWorkService, error) {
	if registry == nil || nextTemplate == nil || accept == nil {
		return nil, ErrInvalidWorkConfig
	}
	return &DevelopmentWorkService{registry: registry, nextTemplate: nextTemplate, accept: accept}, nil
}

func (s *DevelopmentWorkService) GetKawpowWork() (DevelopmentWorkResponse, error) {
	header, err := s.nextTemplate()
	if err != nil {
		return DevelopmentWorkResponse{}, err
	}
	work, err := s.registry.Issue(header)
	if err != nil {
		return DevelopmentWorkResponse{}, err
	}
	return EncodeDevelopmentWork(work), nil
}

func (s *DevelopmentWorkService) SubmitKawpowWork(raw json.RawMessage) (DevelopmentSubmitResponse, error) {
	if len(raw) > MaxDevelopmentSubmissionBytes {
		return rejected("malformed"), nil
	}
	submission, err := DecodeDevelopmentSubmission(raw)
	if err != nil {
		if errors.Is(err, ErrWrongWorkVersion) {
			return rejected("wrong-version"), nil
		}
		return rejected("malformed"), nil
	}
	header, err := s.registry.Submit(submission.WorkID, submission.Nonce, submission.MixDigest)
	if err != nil {
		switch {
		case errors.Is(err, ErrDuplicateWork):
			return rejected("duplicate"), nil
		case errors.Is(err, ErrStaleWork), errors.Is(err, ErrUnknownWork):
			return rejected("stale"), nil
		default:
			return rejected("invalid-seal"), nil
		}
	}
	blockHash, err := s.accept(header)
	if err != nil {
		return DevelopmentSubmitResponse{}, fmt.Errorf("accept verified KawPoW development block: %w", err)
	}
	return DevelopmentSubmitResponse{Accepted: true, Status: "accepted", BlockHash: fixedHex(blockHash[:])}, nil
}

func rejected(status string) DevelopmentSubmitResponse {
	return DevelopmentSubmitResponse{Accepted: false, Status: status}
}
