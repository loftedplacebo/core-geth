//go:build cgo
// +build cgo

package kawpowengine

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func validSubmissionJSON() string {
	return `{"version":"0.1.0-dev","workId":"0x` + strings.Repeat("01", 32) +
		`","nonce":"0x000000000000002a","mixDigest":"0x` + strings.Repeat("ab", 32) + `"}`
}

func TestDevelopmentWorkWireRoundTrip(t *testing.T) {
	work := DevelopmentWork{
		Version: DevelopmentWorkVersion,
		ID:      common.HexToHash("0x01"), HeaderHash: common.HexToHash("0x02"),
		ParentHash: common.HexToHash("0x03"), Height: 42, ExpiresAt: 1_700_000_000,
	}
	work.Target[31] = 0x80
	wire := EncodeDevelopmentWork(work)
	if wire.Version != DevelopmentWorkVersion || wire.Height != "0x2a" || wire.ExpiresAt != "0x6553f100" {
		t.Fatalf("unexpected quantity encoding: %#v", wire)
	}
	for name, value := range map[string]string{"workId": wire.WorkID, "headerHash": wire.HeaderHash, "parentHash": wire.ParentHash, "seedHash": wire.SeedHash, "target": wire.Target} {
		if len(value) != 66 || value[:2] != "0x" || value != strings.ToLower(value) {
			t.Fatalf("%s is not canonical fixed hex: %q", name, value)
		}
	}

	submission, err := DecodeDevelopmentSubmission(json.RawMessage(validSubmissionJSON()))
	if err != nil {
		t.Fatal(err)
	}
	if submission.Version != DevelopmentWorkVersion || submission.Nonce != types.EncodeNonce(42) {
		t.Fatalf("unexpected submission: %#v", submission)
	}
	if submission.WorkID != common.HexToHash("0x"+strings.Repeat("01", 32)) || submission.MixDigest != common.HexToHash("0x"+strings.Repeat("ab", 32)) {
		t.Fatalf("fixed fields changed: %#v", submission)
	}
}

func TestDecodeDevelopmentSubmissionRejectsNonCanonicalInput(t *testing.T) {
	valid := validSubmissionJSON()
	tests := map[string]string{
		"unknown field":    strings.Replace(valid, `}`, `,"extra":1}`, 1),
		"duplicate field":  strings.Replace(valid, `}`, `,"nonce":"0x000000000000002a"}`, 1),
		"wrong version":    strings.Replace(valid, DevelopmentWorkVersion, "0.2.0-dev", 1),
		"missing work id":  strings.Replace(valid, `"workId":"0x`+strings.Repeat("01", 32)+`",`, "", 1),
		"short work id":    strings.Replace(valid, strings.Repeat("01", 32), strings.Repeat("01", 31), 1),
		"uppercase hex":    strings.Replace(valid, strings.Repeat("ab", 32), strings.Repeat("AB", 32), 1),
		"missing prefix":   strings.Replace(valid, `"nonce":"0x`, `"nonce":"`, 1),
		"short nonce":      strings.Replace(valid, "000000000000002a", "00000000000002a", 1),
		"non hex":          strings.Replace(valid, strings.Repeat("ab", 32), "gb"+strings.Repeat("ab", 31), 1),
		"non-string nonce": strings.Replace(valid, `"nonce":"0x000000000000002a"`, `"nonce":42`, 1),
		"trailing value":   valid + ` {}`,
		"array":            `[]`,
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeDevelopmentSubmission(json.RawMessage(input))
			if err == nil {
				t.Fatal("invalid submission accepted")
			}
			if name == "wrong version" {
				if !errors.Is(err, ErrWrongWorkVersion) {
					t.Fatalf("error = %v", err)
				}
			} else if !errors.Is(err, ErrMalformedWorkSubmission) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
