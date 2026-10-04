package oscillator

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// triangle generates a triangle wave with adjustable symmetry: every period it rises linearly from -amplitude to +amplitude
// during the first symmetry part of the period and falls back to -amplitude during the rest.
// At 10 samples per period symmetry 0.5 gives -1, -0.6, -0.2, 0.2, 0.6, 1, 0.6, 0.2, -0.2, -0.6 (the classic triangle),
// and symmetry 0.2 gives -1, 0, 1, 0.75, 0.5, 0.25, 0, -0.25, -0.5, -0.75 (fast rise, slow fall).
// Symmetry 1 is the rising sawtooth (see sawtooth) and 0 a falling one, which sounds the same.
// The classic triangle has only odd harmonics like square, but they fall as 1/n^2 instead of 1/n,
// so it sounds soft, close to a sine, and aliases much less than square or sawtooth.
// The further symmetry is from 0.5, the brighter it gets (even harmonics appear and the high ones grow), up to the sawtooth at 0 or 1.
// Both the rising and the falling part average to 0, so unlike pulse it has no DC offset at any symmetry.
type triangle struct {
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
	// phase is the current, internal phase angle in radians
	phase float64
	// phaseStep is the phase increment per one sample, calculated as angular frequency / sample rate
	phaseStep float64
	// peak is the phase in radians where the wave stops rising and starts falling, 2PI * symmetry
	peak float64
	// riseSlope and fallSlope are how much the wave rises or falls per radian of phase: 2*amplitude over the length of that part
	riseSlope float64
	fallSlope float64
}

func NewTriangle(amplitude, frequency float64, samplingRate int, symmetry float64) (Oscillator, error) {
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
	// Unlike pulseWidth, 0 and 1 are allowed: they give a sawtooth, not a constant.
	if math.IsNaN(symmetry) || symmetry < 0 || 1 < symmetry {
		return nil, fmt.Errorf("symmetry must be between 0 and 1, got: %v", symmetry)
	}

	peak := 2 * math.Pi * symmetry
	return &triangle{
		amplitude: amplitude,
		phase:     0,
		phaseStep: angularFrequency(frequency) / float64(samplingRate),
		peak:      peak,
		// At symmetry 0 or 1 one part has zero length, so its slope is infinite (or NaN at zero amplitude).
		// next() never uses it, because the phase is never below a peak of 0 and never reaches a peak of 2PI.
		riseSlope: 2 * amplitude / peak,
		fallSlope: 2 * amplitude / (2*math.Pi - peak),
	}, nil
}

func (t *triangle) next() float64 {
	var value float64
	if t.phase < t.peak {
		value = t.riseSlope*t.phase - t.amplitude
	} else {
		value = t.amplitude - t.fallSlope*(t.phase-t.peak)
	}
	t.phase += t.phaseStep
	if t.phase >= 2*math.Pi {
		t.phase -= 2 * math.Pi
	}
	return value
}

func (t *triangle) nextSignedInt16() int16 {
	return int16(math.Round(t.next() * math.MaxInt16))
}

func (t *triangle) Read(p []byte) (n int, err error) {
	bufferLength := len(p)
	if bufferLength < 2 {
		return 0, io.ErrShortBuffer
	}

	// while index of next element is smaller than the length e.g. (byteIdx=2, byteIdx+1=3, pLength=3)
	byteIdx := 0
	for ; byteIdx+1 < bufferLength; byteIdx += 2 {
		sample := t.nextSignedInt16()
		// converting int16 to uint16 does not change the internal binary representation
		binary.LittleEndian.PutUint16(p[byteIdx:byteIdx+2], uint16(sample))
	}
	return byteIdx, nil
}
