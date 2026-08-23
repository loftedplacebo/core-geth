package kawpow

/*
#cgo CFLAGS: -I${SRCDIR}/cpp-kawpow/include -I${SRCDIR}/cpp-kawpow/lib
#cgo CXXFLAGS: -std=c++11 -I${SRCDIR}/cpp-kawpow/include -I${SRCDIR}/cpp-kawpow/lib
#cgo LDFLAGS: -lstdc++
#include <stdint.h>

int aichain_kawpow_hash(
    int block_number,
    const uint8_t header_hash[32],
    uint64_t nonce,
    uint8_t mix_hash_out[32],
    uint8_t final_hash_out[32]);
int aichain_kawpow_verify(
    int block_number,
    const uint8_t header_hash[32],
    const uint8_t mix_hash[32],
    uint64_t nonce,
    const uint8_t boundary[32]);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// C1Seal contains the candidate-specific fields passed to the pinned KawPoW
// reference verifier. It is deliberately separate from a consensus header.
type C1Seal struct {
	BlockNumber int
	HeaderHash  [32]byte
	MixHash     [32]byte
	Nonce       uint64
	Boundary    [32]byte
}

// C1Hash computes a CPU reference result for the pinned C1 implementation.
// It is an experimental helper only; it does not provide mining support.
func C1Hash(blockNumber int, headerHash [32]byte, nonce uint64) (mixHash, finalHash [32]byte, err error) {
	if blockNumber < 0 {
		return mixHash, finalHash, fmt.Errorf("block number must be non-negative")
	}
	ok := C.aichain_kawpow_hash(
		C.int(blockNumber),
		(*C.uint8_t)(unsafe.Pointer(&headerHash[0])),
		C.uint64_t(nonce),
		(*C.uint8_t)(unsafe.Pointer(&mixHash[0])),
		(*C.uint8_t)(unsafe.Pointer(&finalHash[0])),
	)
	if ok == 0 {
		return mixHash, finalHash, fmt.Errorf("could not create KawPoW epoch context")
	}
	return mixHash, finalHash, nil
}

// VerifyC1Seal verifies a candidate C1 seal on CPU. A GPU is never required
// for this full-node verification path.
func VerifyC1Seal(seal C1Seal) (bool, error) {
	if seal.BlockNumber < 0 {
		return false, fmt.Errorf("block number must be non-negative")
	}
	return C.aichain_kawpow_verify(
		C.int(seal.BlockNumber),
		(*C.uint8_t)(unsafe.Pointer(&seal.HeaderHash[0])),
		(*C.uint8_t)(unsafe.Pointer(&seal.MixHash[0])),
		C.uint64_t(seal.Nonce),
		(*C.uint8_t)(unsafe.Pointer(&seal.Boundary[0])),
	) != 0, nil
}
