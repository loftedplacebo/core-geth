// Copyright 2026 The AIChain Authors
// This file is part of the AIChain Core-Geth fork.
//go:build cgo
// +build cgo

package kawpowengine

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	DevelopmentASERTHalfLife        = uint64(1800)
	DevelopmentASERTActivationBlock = uint64(1)
)

var (
	asertOne        = big.NewInt(1)
	asertTwo        = big.NewInt(2)
	asertTwo256     = new(big.Int).Lsh(big.NewInt(1), 256)
	asertMaxUint256 = new(big.Int).Sub(new(big.Int).Set(asertTwo256), asertOne)
)

type DevelopmentASERTConfig struct {
	TargetSeconds   uint64
	HalfLifeSeconds uint64
	ActivationBlock uint64
}

func NewDevelopmentASERTConfig(targetSeconds uint64) (*DevelopmentASERTConfig, error) {
	switch targetSeconds {
	case 5, 10, 15:
	default:
		return nil, fmt.Errorf("unsupported development ASERT target %d: want 5, 10, or 15 seconds", targetSeconds)
	}
	return &DevelopmentASERTConfig{
		TargetSeconds:   targetSeconds,
		HalfLifeSeconds: DevelopmentASERTHalfLife,
		ActivationBlock: DevelopmentASERTActivationBlock,
	}, nil
}

func (config *DevelopmentASERTConfig) Validate() error {
	if config == nil {
		return errors.New("nil development ASERT configuration")
	}
	if config.HalfLifeSeconds != DevelopmentASERTHalfLife {
		return fmt.Errorf("development ASERT half-life %d is not the selected %d seconds", config.HalfLifeSeconds, DevelopmentASERTHalfLife)
	}
	if config.ActivationBlock != DevelopmentASERTActivationBlock {
		return fmt.Errorf("development ASERT activation block %d is not the required genesis-child boundary %d", config.ActivationBlock, DevelopmentASERTActivationBlock)
	}
	switch config.TargetSeconds {
	case 5, 10, 15:
		return nil
	default:
		return fmt.Errorf("unsupported development ASERT target %d", config.TargetSeconds)
	}
}

func (config *DevelopmentASERTConfig) Calculate(chain consensus.ChainHeaderReader, timestamp uint64, parent *types.Header) (*big.Int, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if parent == nil || parent.Number == nil || parent.Difficulty == nil || parent.Difficulty.Cmp(asertTwo) < 0 {
		return nil, errors.New("invalid ASERT parent header")
	}
	if timestamp <= parent.Time {
		return nil, errors.New("ASERT candidate timestamp must exceed parent timestamp")
	}
	candidateHeight := new(big.Int).Add(parent.Number, asertOne)
	if candidateHeight.Cmp(new(big.Int).SetUint64(config.ActivationBlock)) < 0 {
		return nil, errors.New("development ASERT calculator called before activation")
	}
	anchorHeight := config.ActivationBlock - 1
	var anchor *types.Header
	if parent.Number.IsUint64() && parent.Number.Uint64() == anchorHeight {
		anchor = parent
	} else if chain != nil {
		anchor = chain.GetHeaderByNumber(anchorHeight)
	}
	if anchor == nil || anchor.Number == nil || anchor.Difficulty == nil || anchor.Difficulty.Cmp(asertTwo) < 0 {
		return nil, errors.New("development ASERT anchor is unavailable or invalid")
	}
	anchorTarget := new(big.Int).Div(new(big.Int).Set(asertTwo256), anchor.Difficulty)
	target, err := calculateASERTTarget(config, anchorTarget, anchor.Number, anchor.Time, candidateHeight, timestamp)
	if err != nil {
		return nil, err
	}
	difficulty := new(big.Int).Div(new(big.Int).Set(asertTwo256), target)
	if difficulty.Cmp(asertTwo) < 0 {
		difficulty.Set(asertTwo)
	}
	if difficulty.Cmp(asertMaxUint256) > 0 {
		difficulty.Set(asertMaxUint256)
	}
	return difficulty, nil
}

func calculateASERTTarget(config *DevelopmentASERTConfig, anchorTarget, anchorHeight *big.Int, anchorTime uint64, candidateHeight *big.Int, candidateTime uint64) (*big.Int, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if anchorTarget == nil || anchorTarget.Sign() <= 0 || anchorHeight == nil || candidateHeight == nil {
		return nil, errors.New("invalid ASERT anchor or candidate")
	}
	heightDelta := new(big.Int).Sub(new(big.Int).Set(candidateHeight), anchorHeight)
	if heightDelta.Sign() <= 0 {
		return nil, errors.New("ASERT candidate height must exceed anchor height")
	}
	timeDelta := new(big.Int).Sub(new(big.Int).SetUint64(candidateTime), new(big.Int).SetUint64(anchorTime))
	idealTime := new(big.Int).Mul(heightDelta, new(big.Int).SetUint64(config.TargetSeconds))
	scheduleError := new(big.Int).Sub(timeDelta, idealTime)
	limit := new(big.Int).SetUint64(256 * config.HalfLifeSeconds)
	if scheduleError.Cmp(limit) >= 0 {
		return new(big.Int).Div(new(big.Int).Set(asertTwo256), asertTwo), nil
	}
	if scheduleError.Cmp(new(big.Int).Neg(limit)) <= 0 {
		return new(big.Int).Set(asertOne), nil
	}
	scaled := new(big.Int).Mul(scheduleError, big.NewInt(65536))
	exponent := floorDivBig(scaled, new(big.Int).SetUint64(config.HalfLifeSeconds)).Int64()
	shifts := floorDivInt64(exponent, 65536)
	fraction := exponent - shifts*65536

	f := big.NewInt(fraction)
	f2 := new(big.Int).Mul(f, f)
	f3 := new(big.Int).Mul(f2, f)
	polynomial := new(big.Int).Mul(big.NewInt(195766423245049), f)
	polynomial.Add(polynomial, new(big.Int).Mul(big.NewInt(971821376), f2))
	polynomial.Add(polynomial, new(big.Int).Mul(big.NewInt(5127), f3))
	polynomial.Add(polynomial, new(big.Int).Lsh(big.NewInt(1), 47))
	polynomial.Rsh(polynomial, 48)
	factor := new(big.Int).Add(big.NewInt(65536), polynomial)

	target := new(big.Int).Mul(new(big.Int).Set(anchorTarget), factor)
	shifts -= 16
	if shifts < 0 {
		target.Rsh(target, uint(-shifts))
	} else {
		target.Lsh(target, uint(shifts))
	}
	maxTarget := new(big.Int).Div(new(big.Int).Set(asertTwo256), asertTwo)
	if target.Sign() <= 0 {
		target.Set(asertOne)
	}
	if target.Cmp(maxTarget) > 0 {
		target.Set(maxTarget)
	}
	return target, nil
}

func floorDivBig(numerator, denominator *big.Int) *big.Int {
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if remainder.Sign() != 0 && numerator.Sign() < 0 {
		quotient.Sub(quotient, asertOne)
	}
	return quotient
}

func floorDivInt64(numerator, denominator int64) int64 {
	quotient, remainder := numerator/denominator, numerator%denominator
	if remainder != 0 && ((remainder < 0) != (denominator < 0)) {
		quotient--
	}
	return quotient
}
