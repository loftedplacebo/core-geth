package firopow

/*
#cgo CFLAGS: -I${SRCDIR}/firo/src
#cgo CXXFLAGS: -std=c++11 -I${SRCDIR}/firo/src
#cgo LDFLAGS: -lstdc++
#include <stdint.h>

int aichain_firopow_hash(
    int block_number,
    const uint8_t header_hash[32],
    uint64_t nonce,
    uint8_t mix_hash_out[32],
    uint8_t final_hash_out[32]);
int aichain_firopow_verify(
    int block_number,
    const uint8_t header_hash[32],
    const uint8_t mix_hash[32],
    uint64_t nonce,
    const uint8_t boundary[32]);
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

const maxC2BlockNumber = uint64(^uint32(0) >> 1)

var ErrUnsupportedBlockNumber = errors.New("FiroPoW candidate block number is outside the pinned reference range")

// C2Seal is the FiroPoW reference-verifier input. It is deliberately not a
// Core-Geth consensus header or engine input.
type C2Seal struct {
	BlockNumber int
	HeaderHash  [32]byte
	MixHash     [32]byte
	Nonce       uint64
	Boundary    [32]byte
}

// C2Hash calculates an official FiroPoW reference result on CPU. It is not a
// mining interface.
func C2Hash(blockNumber int, headerHash [32]byte, nonce uint64) (mixHash, finalHash [32]byte, err error) {
	if blockNumber < 0 || uint64(blockNumber) > maxC2BlockNumber {
		return mixHash, finalHash, ErrUnsupportedBlockNumber
	}
	ok := C.aichain_firopow_hash(
		C.int(blockNumber),
		(*C.uint8_t)(unsafe.Pointer(&headerHash[0])),
		C.uint64_t(nonce),
		(*C.uint8_t)(unsafe.Pointer(&mixHash[0])),
		(*C.uint8_t)(unsafe.Pointer(&finalHash[0])),
	)
	if ok == 0 {
		return mixHash, finalHash, fmt.Errorf("could not create FiroPoW epoch context")
	}
	return mixHash, finalHash, nil
}

// VerifyC2Seal performs CPU-side candidate verification through Firo's pinned
// reference code. Full-node validation must never require a GPU.
func VerifyC2Seal(seal C2Seal) (bool, error) {
	if seal.BlockNumber < 0 || uint64(seal.BlockNumber) > maxC2BlockNumber {
		return false, ErrUnsupportedBlockNumber
	}
	return C.aichain_firopow_verify(
		C.int(seal.BlockNumber),
		(*C.uint8_t)(unsafe.Pointer(&seal.HeaderHash[0])),
		(*C.uint8_t)(unsafe.Pointer(&seal.MixHash[0])),
		C.uint64_t(seal.Nonce),
		(*C.uint8_t)(unsafe.Pointer(&seal.Boundary[0])),
	) != 0, nil
}
