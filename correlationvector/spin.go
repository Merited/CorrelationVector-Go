// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

// Package correlationvector contains library functions to manipulate CorrelationVectors.
package correlationvector

import (
	"math/rand"
	"strconv"
	"time"
)

// SpinCounterInterval is the interval (proportional to time) by which the counter increments.
type SpinCounterInterval int

const (
	// CoarseInterval drops the 24 least significant bits in time.Now().NanoSeconds() / 100
	// resulting in a counter that increments every 1.67 seconds.
	CoarseInterval SpinCounterInterval = iota

	// FineInterval drops the 16 least significant bits in time.Now().NanoSeconds() / 100
	// resulting in a counter that increments every 6.5 milliseconds.
	FineInterval SpinCounterInterval = iota
)

// SpinCounterPeriodicity configures how frequently the counter wraps around to zero,
// as determined by the amount of space to store the counter.
type SpinCounterPeriodicity int

const (
	// NoPeriodicity does not store a counter as part of the spin value.
	NoPeriodicity SpinCounterPeriodicity = iota

	// ShortPeriodicity stores the counter using 16 bits.
	ShortPeriodicity SpinCounterPeriodicity = iota

	// MediumPeriodicity stores the counter using 24 bits.
	MediumPeriodicity SpinCounterPeriodicity = iota

	// LongPeriodicity stores the counter using 32 bits.
	LongPeriodicity SpinCounterPeriodicity = iota
)

// SpinEntropy is the number of bytes to use for entropy. Valid values from a
// minimum of 0 to a maximum of 4.
type SpinEntropy int

const (
	// NoEntropy does not generate entropy as part of the spin value.
	NoEntropy SpinEntropy = 0

	// OneEntropy generate entropy using 8 bits.
	OneEntropy SpinEntropy = 1

	// TwoEntropy generate entropy using 16 bits.
	TwoEntropy SpinEntropy = 2

	// ThreeEntropy generate entropy using 24 bits.
	ThreeEntropy SpinEntropy = 3

	// FourEntropy generate entropy using 32 bits.
	FourEntropy SpinEntropy = 4
)

// SpinParameters stores parameters used by the CorrelationVector Spin operator.
type SpinParameters struct {
	Interval    SpinCounterInterval
	Periodicity SpinCounterPeriodicity
	Entropy     SpinEntropy
}

// Spin creates a new correlation vector by applying the Spin operator to an
// existing value. This should be done at the entry point of an operation.
func Spin(correlationVector string) (*CorrelationVector, error) {
	return SpinWithParameters(correlationVector, &defaultParameters)
}

// SpinWithParameters creates a new correlation vector by applying the Spin
// operator to an existing value. This should be done at the entry point of an operation.
func SpinWithParameters(correlationVector string, parameters *SpinParameters) (*CorrelationVector, error) {
	version, err := inferVersion(correlationVector)
	if err != nil {
		return nil, err
	}

	if ValidateCorrelationVectorDuringCreation {
		if err = validate(correlationVector, version); err != nil {
			return nil, err
		}
	}

	entropy := make([]byte, int(parameters.Entropy))
	rand.Read(entropy)

	// Ticks is defined as 100 nanoseconds.
	ticks := time.Now().UnixNano() / 100

	value := uint64(ticks >> parameters.tickBitsToDrop())
	for i := 0; i < int(parameters.Entropy); i++ {
		value = (value << 8) | uint64(entropy[i])
	}

	// Generate a bitmask and mask the lower totalBits in the value.
	// The mask is generated by (1 << totalBits) - 1. We need to handle the edge case
	// when shifting 64 bits, as it wraps around.
	mask := uint64(1) << parameters.totalBits()
	if parameters.totalBits() == 64 {
		mask = 0
	}
	mask -= 1
	value &= mask

	s := strconv.Itoa(int(value))
	if parameters.totalBits() > 32 {
		s = strconv.Itoa(int(value>>32)) + "." + s
	}

	return newCorrelationVector(correlationVector+"."+s, 0, version), nil
}

var defaultParameters = SpinParameters{CoarseInterval, ShortPeriodicity, TwoEntropy}

func (sp *SpinParameters) tickBitsToDrop() uint {
	switch sp.Interval {
	case CoarseInterval:
		return 24
	case FineInterval:
		return 16
	default:
		return 24
	}
}

func (sp *SpinParameters) totalBits() uint {
	counterBits := uint(0)
	switch sp.Periodicity {
	case ShortPeriodicity:
		counterBits = 16
	case MediumPeriodicity:
		counterBits = 24
	case LongPeriodicity:
		counterBits = 32
	}

	return counterBits + uint(sp.Entropy)*8
}
