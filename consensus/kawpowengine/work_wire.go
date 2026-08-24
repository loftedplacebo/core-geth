// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//go:build cgo
// +build cgo

package kawpowengine

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/kawpow"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	ErrMalformedWorkSubmission = errors.New("malformed KawPoW work submission")
	ErrWrongWorkVersion        = errors.New("wrong KawPoW work protocol version")
)

// DevelopmentWorkResponse is the canonical JSON-facing representation of a
// node-issued work record. Fixed-size values are always lowercase full-width
// hexadecimal strings; quantities use Ethereum JSON quantity encoding.
type DevelopmentWorkResponse struct {
	Version    string `json:"version"`
	WorkID     string `json:"workId"`
	HeaderHash string `json:"headerHash"`
	ParentHash string `json:"parentHash"`
	Height     string `json:"height"`
	SeedHash   string `json:"seedHash"`
	Target     string `json:"target"`
	ExpiresAt  string `json:"expiresAt"`
}

type DevelopmentSubmission struct {
	Version   string
	WorkID    common.Hash
	Nonce     types.BlockNonce
	MixDigest common.Hash
}

func EncodeDevelopmentWork(work DevelopmentWork) DevelopmentWorkResponse {
	seedHash := kawpow.SeedHash(work.Height)
	return DevelopmentWorkResponse{
		Version:    work.Version,
		WorkID:     fixedHex(work.ID[:]),
		HeaderHash: fixedHex(work.HeaderHash[:]),
		ParentHash: fixedHex(work.ParentHash[:]),
		Height:     hexutil.EncodeUint64(work.Height),
		SeedHash:   fixedHex(seedHash[:]),
		Target:     fixedHex(work.Target[:]),
		ExpiresAt:  hexutil.EncodeUint64(work.ExpiresAt),
	}
}

// DecodeDevelopmentSubmission performs strict decoding before a submission can
// reach the work registry. It rejects unknown fields and non-canonical fixed
// hexadecimal values rather than relying on permissive common.Hash decoding.
func DecodeDevelopmentSubmission(raw json.RawMessage) (DevelopmentSubmission, error) {
	fields, err := decodeSubmissionObject(raw)
	if err != nil {
		return DevelopmentSubmission{}, err
	}
	version, err := decodeRequiredString(fields, "version")
	if err != nil {
		return DevelopmentSubmission{}, err
	}
	if version != DevelopmentWorkVersion {
		return DevelopmentSubmission{}, ErrWrongWorkVersion
	}
	workIDText, err := decodeRequiredString(fields, "workId")
	if err != nil {
		return DevelopmentSubmission{}, err
	}
	workID, err := decodeFixedHex("workId", workIDText, common.HashLength)
	if err != nil {
		return DevelopmentSubmission{}, err
	}
	nonceText, err := decodeRequiredString(fields, "nonce")
	if err != nil {
		return DevelopmentSubmission{}, err
	}
	nonce, err := decodeFixedHex("nonce", nonceText, len(types.BlockNonce{}))
	if err != nil {
		return DevelopmentSubmission{}, err
	}
	mixDigestText, err := decodeRequiredString(fields, "mixDigest")
	if err != nil {
		return DevelopmentSubmission{}, err
	}
	mixDigest, err := decodeFixedHex("mixDigest", mixDigestText, common.HashLength)
	if err != nil {
		return DevelopmentSubmission{}, err
	}
	var result DevelopmentSubmission
	result.Version = version
	copy(result.WorkID[:], workID)
	copy(result.Nonce[:], nonce)
	copy(result.MixDigest[:], mixDigest)
	return result, nil
}

func decodeSubmissionObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("%w: submission must be one JSON object", ErrMalformedWorkSubmission)
	}
	fields := make(map[string]json.RawMessage, 4)
	allowed := map[string]bool{"version": true, "workId": true, "nonce": true, "mixDigest": true}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformedWorkSubmission, err)
		}
		key, ok := keyToken.(string)
		if !ok || !allowed[key] {
			return nil, fmt.Errorf("%w: unknown field", ErrMalformedWorkSubmission)
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, fmt.Errorf("%w: duplicate field %s", ErrMalformedWorkSubmission, key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformedWorkSubmission, err)
		}
		fields[key] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, fmt.Errorf("%w: incomplete submission object", ErrMalformedWorkSubmission)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, err
	}
	if len(fields) != 4 {
		return nil, fmt.Errorf("%w: all fields are required", ErrMalformedWorkSubmission)
	}
	return fields, nil
}

func decodeRequiredString(fields map[string]json.RawMessage, field string) (string, error) {
	raw, ok := fields[field]
	if !ok {
		return "", fmt.Errorf("%w: missing %s", ErrMalformedWorkSubmission, field)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%w: %s must be a string", ErrMalformedWorkSubmission, field)
	}
	return value, nil
}

func decodeFixedHex(field, input string, size int) ([]byte, error) {
	if len(input) != 2+size*2 || len(input) < 2 || input[0:2] != "0x" {
		return nil, fmt.Errorf("%w: %s must be 0x-prefixed and exactly %d bytes", ErrMalformedWorkSubmission, field, size)
	}
	for _, char := range input[2:] {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return nil, fmt.Errorf("%w: %s is not canonical lowercase hexadecimal", ErrMalformedWorkSubmission, field)
		}
	}
	decoded, err := hex.DecodeString(input[2:])
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrMalformedWorkSubmission, field, err)
	}
	return decoded, nil
}

func fixedHex(value []byte) string {
	return "0x" + hex.EncodeToString(value)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%w: trailing JSON value", ErrMalformedWorkSubmission)
		}
		return fmt.Errorf("%w: %v", ErrMalformedWorkSubmission, err)
	}
	return nil
}
