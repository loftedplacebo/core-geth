//go:build cgo
// +build cgo

package kawpowengine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

func testWorkService(t *testing.T, verify func(*types.Header) error, accept func(*types.Header) (common.Hash, error)) (*DevelopmentWorkService, *types.Header) {
	t.Helper()
	var canonical atomic.Bool
	canonical.Store(true)
	registry := testRegistry(t, &canonical, verify)
	template := workHeader(1, 42)
	service, err := NewDevelopmentWorkService(registry, func() (*types.Header, error) {
		return types.CopyHeader(template), nil
	}, accept)
	if err != nil {
		t.Fatal(err)
	}
	return service, template
}

func submissionForWork(work DevelopmentWorkResponse, nonce, mix string) json.RawMessage {
	return json.RawMessage(`{"version":"` + DevelopmentWorkVersion + `","workId":"` + work.WorkID + `","nonce":"` + nonce + `","mixDigest":"` + mix + `"}`)
}

func TestDevelopmentWorkServiceThroughInMemoryRPC(t *testing.T) {
	acceptedHash := common.HexToHash("0x1234")
	service, _ := testWorkService(t, func(*types.Header) error { return nil }, func(*types.Header) (common.Hash, error) {
		return acceptedHash, nil
	})
	server := rpc.NewServer()
	if err := server.RegisterName("aichain", service); err != nil {
		t.Fatal(err)
	}
	client := rpc.DialInProc(server)
	defer client.Close()

	var work DevelopmentWorkResponse
	if err := client.Call(&work, "aichain_getKawpowWork"); err != nil {
		t.Fatal(err)
	}
	var result DevelopmentSubmitResponse
	if err := client.Call(&result, "aichain_submitKawpowWork", submissionForWork(work, "0x000000000000002a", "0x"+strings.Repeat("ab", 32))); err != nil {
		t.Fatal(err)
	}
	if !result.Accepted || result.Status != "accepted" || result.BlockHash != fixedHex(acceptedHash[:]) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestDevelopmentWorkServiceRejectionStatuses(t *testing.T) {
	invalidSeal := errors.New("invalid seal")
	service, _ := testWorkService(t, func(*types.Header) error { return invalidSeal }, func(h *types.Header) (common.Hash, error) { return h.Hash(), nil })
	work, err := service.GetKawpowWork(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{"malformed", json.RawMessage(`{}`), "malformed"},
		{"wrong version", json.RawMessage(strings.Replace(string(submissionForWork(work, "0x000000000000002a", "0x"+strings.Repeat("ab", 32))), DevelopmentWorkVersion, "0.2.0-dev", 1)), "wrong-version"},
		{"unknown", submissionForWork(DevelopmentWorkResponse{WorkID: "0x" + strings.Repeat("00", 32)}, "0x000000000000002a", "0x"+strings.Repeat("ab", 32)), "stale"},
		{"invalid seal", submissionForWork(work, "0x000000000000002a", "0x"+strings.Repeat("ab", 32)), "invalid-seal"},
		{"oversize", json.RawMessage(`{"pad":"` + strings.Repeat("x", MaxDevelopmentSubmissionBytes) + `"}`), "malformed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := service.SubmitKawpowWork(context.Background(), test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got.Accepted || got.Status != test.want || got.BlockHash != "" {
				t.Fatalf("result = %#v, want status %s", got, test.want)
			}
		})
	}
}

func TestDevelopmentTransportPolicy(t *testing.T) {
	tests := []struct {
		name string
		info rpc.PeerInfo
		want bool
	}{
		{"in process", rpc.PeerInfo{}, true},
		{"ipc", rpc.PeerInfo{Transport: "ipc", RemoteAddr: "/tmp/aichain.ipc"}, true},
		{"loopback http", rpc.PeerInfo{Transport: "http", RemoteAddr: "127.0.0.1:1234"}, true},
		{"loopback ipv6", rpc.PeerInfo{Transport: "ws", RemoteAddr: "[::1]:1234"}, true},
		{"remote http", rpc.PeerInfo{Transport: "http", RemoteAddr: "198.51.100.2:1234"}, false},
		{"malformed address", rpc.PeerInfo{Transport: "http", RemoteAddr: "localhost"}, false},
		{"unknown transport", rpc.PeerInfo{Transport: "stdio"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsDevelopmentTransportAllowed(test.info); got != test.want {
				t.Fatalf("allowed = %t, want %t", got, test.want)
			}
		})
	}
}

func TestDevelopmentWorkServiceRateLimits(t *testing.T) {
	service, _ := testWorkService(t, func(*types.Header) error { return nil }, func(h *types.Header) (common.Hash, error) { return h.Hash(), nil })
	for i := 0; i < developmentGetBurst; i++ {
		if _, err := service.GetKawpowWork(context.Background()); err != nil {
			t.Fatalf("request %d rejected inside burst: %v", i, err)
		}
	}
	if _, err := service.GetKawpowWork(context.Background()); !errors.Is(err, ErrDevelopmentRateLimited) {
		t.Fatalf("burst overflow error = %v, want %v", err, ErrDevelopmentRateLimited)
	}
}

func TestDevelopmentWorkServiceDoesNotRegisterItself(t *testing.T) {
	service, _ := testWorkService(t, func(*types.Header) error { return nil }, func(h *types.Header) (common.Hash, error) { return h.Hash(), nil })
	if service == nil {
		t.Fatal("service not constructed")
	}
	server := rpc.NewServer()
	client := rpc.DialInProc(server)
	defer client.Close()
	var result DevelopmentWorkResponse
	if err := client.Call(&result, "aichain_getKawpowWork"); err == nil {
		t.Fatal("unregistered development service was reachable")
	}
}
