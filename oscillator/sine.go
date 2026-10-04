package oscillator

import (
	"encoding/binary"
	"fmt"
	"math"
)

// sine generates sinusoidal waveforms for audio synthesis.
// It maintains phase information to produce continuous sine waves at a specified frequency.
type sine struct {
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
	// frequency is the Oscillator's frequency in Hz
	frequency uint
	// phase is the current, internal phase angle in radians
	phase float64
	// phaseStep is the phase increment per one sample, calculated as angular frequency / sample rate
	phaseStep float64
}

func NewSine(amplitude float64, frequency uint, samplingRate int) (Oscillator, error) {
	if amplitude < 0 || 1 < amplitude {
		return nil, fmt.Errorf("amplitude must be between 0 and 1, got: %v", amplitude)
	}
	if frequency == 0 {
		return nil, fmt.Errorf("frequency must be positive, got %v", frequency)
	}
	if samplingRate <= 0 {
		return nil, fmt.Errorf("samplingRate must be positive, got: %v", samplingRate)
	}

	// angularFrequency is the Oscillator's angular frequency in radians per second
	angFreq := angularFrequency(frequency)
	return &sine{
		amplitude: amplitude,
		frequency: frequency,
		phase:     0,
		phaseStep: angFreq / float64(samplingRate),
	}, nil
}

// todo: test
// todo: benchmark
func (s *sine) next() float64 {
	value := s.amplitude * math.Sin(s.phase)
	s.phase += s.phaseStep
	if s.phase >= 2*math.Pi {
		s.phase -= 2 * math.Pi
	}
	return value
}

func (s *sine) nextSignedInt16() int16 {
	return int16(math.Round(s.next() * math.MaxInt16))
}

func (s *sine) Read(p []byte) (n int, err error) {
	bufferLength := len(p)

	// while index of next element is smaller than the length e.g. (byteIdx=2, byteIdx+1=3, pLength=3)
	byteIdx := 0
	for ; byteIdx+1 < bufferLength; byteIdx += 2 {
		sample := s.nextSignedInt16()
		// converting int16 to uint16 does not change the internal binary representation
		binary.LittleEndian.PutUint16(p[byteIdx:byteIdx+2], uint16(sample))
	}
	return byteIdx, nil
}
