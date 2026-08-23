//go:build cgo
// +build cgo

package kawpowengine

import (
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"testing"
)

func TestDevelopmentEngineDisablesMining(t *testing.T) {
	e := New(ethash.Config{})
	if err := e.Seal(nil, nil, nil, nil); err != ErrMiningUnavailable {
		t.Fatalf("Seal error = %v", err)
	}
	if e.Hashrate() != 0 {
		t.Fatal("development engine reports a hashrate")
	}
}
