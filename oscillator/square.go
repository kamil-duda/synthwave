package oscillator

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// square generates a naive square wave: +amplitude in the first half of every period and -amplitude in the second.
// An ideal square wave is a sum of infinitely many odd harmonics (f, 3f, 5f, ...) with amplitudes falling as 1/n.
// Sampling cannot represent the harmonics above the Nyquist frequency, so they alias: each one folds back below it as a tone
// that is not a multiple of f (at 44.1 kHz the 101st harmonic of 440 Hz, 44440 Hz, is heard at 340 Hz with 1/101 of the amplitude).
// Low notes alias quietly, because only weak, high harmonics fold back. High notes fold back strong ones and sound out of tune.
// A band-limited square (e.g. PolyBLEP) avoids this by rounding off each jump over a couple of samples,
// which removes most of the harmonics above the Nyquist frequency.
type square struct {
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
	// phase is the current, internal phase angle in radians
	phase float64
	// phaseStep is the phase increment per one sample, calculated as angular frequency / sample rate
	phaseStep float64
}

func NewSquare(amplitude float64, frequency uint, samplingRate int) (Oscillator, error) {
	if math.IsNaN(amplitude) || amplitude < 0 || 1 < amplitude {
		return nil, fmt.Errorf("amplitude must be between 0 and 1, got: %v", amplitude)
	}
	if frequency == 0 {
		return nil, fmt.Errorf("frequency must be positive, got %v", frequency)
	}
	if samplingRate <= 0 {
		return nil, fmt.Errorf("samplingRate must be positive, got: %v", samplingRate)
	}
	// The Nyquist check (explained in NewSine) only covers the fundamental frequency f, the harmonics above it alias anyway (see square).
	// It still matters: a fundamental at or above the Nyquist frequency aliases just like a sine,
	// and staying below it keeps phaseStep < PI, so next() never needs to wrap the phase more than once.
	nyquistFrequency := float64(samplingRate) / 2
	if float64(frequency) >= nyquistFrequency {
		return nil, fmt.Errorf("frequency must be below the Nyquist frequency %v Hz, got: %v", nyquistFrequency, frequency)
	}

	return &square{
		amplitude: amplitude,
		phase:     0,
		phaseStep: angularFrequency(frequency) / float64(samplingRate),
	}, nil
}

func (s *square) next() float64 {
	value := s.amplitude
	if s.phase >= math.Pi {
		value = -s.amplitude
	}
	s.phase += s.phaseStep
	if s.phase >= 2*math.Pi {
		s.phase -= 2 * math.Pi
	}
	return value
}

func (s *square) nextSignedInt16() int16 {
	return int16(math.Round(s.next() * math.MaxInt16))
}

func (s *square) Read(p []byte) (n int, err error) {
	bufferLength := len(p)
	if bufferLength < 2 {
		return 0, io.ErrShortBuffer
	}

	// while index of next element is smaller than the length e.g. (byteIdx=2, byteIdx+1=3, pLength=3)
	byteIdx := 0
	for ; byteIdx+1 < bufferLength; byteIdx += 2 {
		sample := s.nextSignedInt16()
		// converting int16 to uint16 does not change the internal binary representation
		binary.LittleEndian.PutUint16(p[byteIdx:byteIdx+2], uint16(sample))
	}
	return byteIdx, nil
}
