// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//go:build cgo
// +build cgo

package kawpowengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/time/rate"
)

const MaxDevelopmentSubmissionBytes = 4 * 1024

const (
	developmentMaxRateLimitClients = 64
	developmentGetRate             = 2
	developmentGetBurst            = 4
	developmentSubmitRate          = 20
	developmentSubmitBurst         = 40
)

var (
	ErrLocalTransportRequired = errors.New("KawPoW development RPC requires IPC or a loopback transport")
	ErrDevelopmentRateLimited = errors.New("KawPoW development RPC rate limit exceeded")
)

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
	getLimits    *boundedClientLimiters
	submitLimits *boundedClientLimiters
}

type clientLimiter struct {
	limiter *rate.Limiter
	touched time.Time
}

type boundedClientLimiters struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	max     int
	clients map[string]*clientLimiter
}

func newBoundedClientLimiters(limit rate.Limit, burst, max int) *boundedClientLimiters {
	return &boundedClientLimiters{limit: limit, burst: burst, max: max, clients: make(map[string]*clientLimiter)}
}

func (l *boundedClientLimiters) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	entry := l.clients[key]
	if entry == nil {
		if len(l.clients) >= l.max {
			var oldestKey string
			var oldest time.Time
			for candidate, existing := range l.clients {
				if oldestKey == "" || existing.touched.Before(oldest) {
					oldestKey, oldest = candidate, existing.touched
				}
			}
			delete(l.clients, oldestKey)
		}
		entry = &clientLimiter{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.clients[key] = entry
	}
	entry.touched = now
	return entry.limiter.Allow()
}

func NewDevelopmentWorkService(
	registry *WorkRegistry,
	nextTemplate func() (*types.Header, error),
	accept func(*types.Header) (common.Hash, error),
) (*DevelopmentWorkService, error) {
	if registry == nil || nextTemplate == nil || accept == nil {
		return nil, ErrInvalidWorkConfig
	}
	return &DevelopmentWorkService{
		registry: registry, nextTemplate: nextTemplate, accept: accept,
		getLimits:    newBoundedClientLimiters(developmentGetRate, developmentGetBurst, developmentMaxRateLimitClients),
		submitLimits: newBoundedClientLimiters(developmentSubmitRate, developmentSubmitBurst, developmentMaxRateLimitClients),
	}, nil
}

func (s *DevelopmentWorkService) GetKawpowWork(ctx context.Context) (DevelopmentWorkResponse, error) {
	if err := enforceDevelopmentTransport(ctx); err != nil {
		return DevelopmentWorkResponse{}, err
	}
	if !s.getLimits.allow(developmentClientKey(ctx)) {
		return DevelopmentWorkResponse{}, ErrDevelopmentRateLimited
	}
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

func (s *DevelopmentWorkService) SubmitKawpowWork(ctx context.Context, raw json.RawMessage) (DevelopmentSubmitResponse, error) {
	if err := enforceDevelopmentTransport(ctx); err != nil {
		return DevelopmentSubmitResponse{}, err
	}
	if !s.submitLimits.allow(developmentClientKey(ctx)) {
		return DevelopmentSubmitResponse{}, ErrDevelopmentRateLimited
	}
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

func enforceDevelopmentTransport(ctx context.Context) error {
	if IsDevelopmentTransportAllowed(rpc.PeerInfoFromContext(ctx)) {
		return nil
	}
	return ErrLocalTransportRequired
}

// IsDevelopmentTransportAllowed keeps the development work API off remote
// network transports even if an operator accidentally whitelists its namespace.
func IsDevelopmentTransportAllowed(info rpc.PeerInfo) bool {
	switch info.Transport {
	case "", "ipc": // Empty is the trusted in-process transport used by tests and embedding.
		return true
	case "http", "ws":
		host, _, err := net.SplitHostPort(info.RemoteAddr)
		if err != nil {
			return false
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	default:
		return false
	}
}

func developmentClientKey(ctx context.Context) string {
	info := rpc.PeerInfoFromContext(ctx)
	if info.Transport == "" {
		return "inproc"
	}
	return info.Transport + "|" + info.RemoteAddr
}

func rejected(status string) DevelopmentSubmitResponse {
	return DevelopmentSubmitResponse{Accepted: false, Status: status}
}
