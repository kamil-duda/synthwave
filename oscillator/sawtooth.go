package oscillator

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// sawtooth generates a naive rising sawtooth wave: every period it rises linearly from -amplitude towards +amplitude
// and then jumps back down. At 10 samples per period it gives -1, -0.8, ..., 0.6, 0.8, -1, -0.8, ...
// It is the cheapest waveform here: next() is one multiplication and one subtraction, without math.Sin or a branch
// (apart from the phase wrap that every oscillator does).
// A sawtooth contains every harmonic (f, 2f, 3f, ...) with amplitudes falling as 1/n, which makes it bright and buzzy.
// It aliases more than square, which has only the odd harmonics: each harmonic above the Nyquist frequency folds back below it
// (see square). A band-limited sawtooth (e.g. PolyBLEP) would remove most of that.
// The wave spends as much of the period above 0 as below, so unlike pulse it has no DC offset.
type sawtooth struct {
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
	// phase is the current, internal phase angle in radians
	phase float64
	// phaseStep is the phase increment per one sample, calculated as angular frequency / sample rate
	phaseStep float64
	// slope is how much the wave rises per radian of phase: 2*amplitude over one period of 2PI, i.e. amplitude / PI
	slope float64
}

func NewSawtooth(amplitude, frequency float64, samplingRate int) (Oscillator, error) {
	if math.IsNaN(amplitude) || amplitude < 0 || 1 < amplitude {
		return nil, fmt.Errorf("amplitude must be between 0 and 1, got: %v", amplitude)
	}
	if math.IsNaN(frequency) || frequency <= 0 {
		return nil, fmt.Errorf("frequency must be positive, got %v", frequency)
	}
	if samplingRate <= 0 {
		return nil, fmt.Errorf("samplingRate must be positive, got: %v", samplingRate)
	}
	// The Nyquist check covers only the fundamental frequency f, like in NewSquare (see there and NewSine).
	nyquistFrequency := float64(samplingRate) / 2
	if frequency >= nyquistFrequency {
		return nil, fmt.Errorf("frequency must be below the Nyquist frequency %v Hz, got: %v", nyquistFrequency, frequency)
	}

	return &sawtooth{
		amplitude: amplitude,
		phase:     0,
		phaseStep: angularFrequency(frequency) / float64(samplingRate),
		slope:     amplitude / math.Pi,
	}, nil
}

func (s *sawtooth) next() float64 {
	value := s.slope*s.phase - s.amplitude
	s.phase += s.phaseStep
	if s.phase >= 2*math.Pi {
		s.phase -= 2 * math.Pi
	}
	return value
}

func (s *sawtooth) nextSignedInt16() int16 {
	return int16(math.Round(s.next() * math.MaxInt16))
}

func (s *sawtooth) Read(p []byte) (n int, err error) {
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
