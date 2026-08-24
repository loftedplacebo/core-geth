// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//go:build cgo
// +build cgo

package eth

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/kawpowengine"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/types/genesisT"
)

func newKawpowDevelopmentTestNode(t *testing.T, enabled bool) (*node.Node, *Ethereum) {
	t.Helper()
	stack, err := node.New(&node.Config{P2P: p2p.Config{ListenAddr: "127.0.0.1:0", NoDiscovery: true, MaxPeers: 0}})
	if err != nil {
		t.Fatal(err)
	}
	cfg := ethconfig.Defaults
	cfg.Genesis = &genesisT.Genesis{
		Config:     params.AllEthashProtocolChanges,
		Difficulty: big.NewInt(2),
		GasLimit:   30_000_000,
		Timestamp:  uint64(time.Now().Add(-time.Second).Unix()),
	}
	cfg.SyncMode = downloader.FullSync
	cfg.Ethash.PowMode = ethash.ModeNormal
	cfg.KawpowDevelopment = enabled
	cfg.Miner.Etherbase = common.HexToAddress("0x0000000000000000000000000000000000000001")
	service, err := New(stack, &cfg)
	if err != nil {
		stack.Close()
		t.Fatal(err)
	}
	if err := stack.Start(); err != nil {
		stack.Close()
		t.Fatal(err)
	}
	if enabled {
		if err := service.StartMining(0); err != nil {
			stack.Close()
			t.Fatal(err)
		}
	}
	return stack, service
}

func TestKawpowDevelopmentAPIDisabledByDefault(t *testing.T) {
	stack, _ := newKawpowDevelopmentTestNode(t, false)
	defer stack.Close()
	client := stack.Attach()
	defer client.Close()
	var work kawpowengine.DevelopmentWorkResponse
	if err := client.Call(&work, "aichain_getKawpowWork"); err == nil {
		t.Fatal("KawPoW development API was registered without opt-in")
	}
}

func TestKawpowDevelopmentAPIUsesRealPendingTemplate(t *testing.T) {
	stack, service := newKawpowDevelopmentTestNode(t, true)
	defer stack.Close()
	client := stack.Attach()
	defer client.Close()

	var (
		work kawpowengine.DevelopmentWorkResponse
		err  error
	)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		err = client.Call(&work, "aichain_getKawpowWork")
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("get development work: %v", err)
	}
	current := service.BlockChain().CurrentHeader()
	if work.Version != kawpowengine.DevelopmentWorkVersion || work.Height != "0x1" || work.ParentHash != strings.ToLower(current.Hash().Hex()) {
		t.Fatalf("work does not describe the real pending child: %#v", work)
	}
	if work.HeaderHash == (common.Hash{}).Hex() || len(work.Target) != 66 {
		t.Fatalf("work is missing seal inputs: %#v", work)
	}

	var malformed kawpowengine.DevelopmentSubmitResponse
	if err := client.Call(&malformed, "aichain_submitKawpowWork", 1); err != nil {
		t.Fatalf("malformed submission returned transport error: %v", err)
	}
	if malformed.Accepted || malformed.Status != "malformed" {
		t.Fatalf("malformed submission result = %#v", malformed)
	}

	raw := `{"version":"` + kawpowengine.DevelopmentWorkVersion + `","workId":"` + work.WorkID + `","nonce":"0x0000000000000000","mixDigest":"0x` + strings.Repeat("00", 32) + `"}`
	var invalid kawpowengine.DevelopmentSubmitResponse
	if err := client.Call(&invalid, "aichain_submitKawpowWork", json.RawMessage(raw)); err != nil {
		t.Fatalf("invalid seal returned transport error: %v", err)
	}
	if invalid.Accepted || invalid.Status != "invalid-seal" {
		t.Fatalf("invalid seal result = %#v", invalid)
	}
	if service.BlockChain().CurrentHeader().Number.Sign() != 0 {
		t.Fatal("invalid development work changed the canonical chain")
	}
}
