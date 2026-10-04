package oscillator

import (
	"fmt"
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
	phaseTracker
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
	// symmetry is the part of the period the wave rises, kept for String
	symmetry float64
	// peak is the phase in radians where the wave stops rising and starts falling, 2PI * symmetry
	peak float64
	// riseSlope and fallSlope are how much the wave rises or falls per radian of phase: 2*amplitude over the length of that part
	riseSlope float64
	fallSlope float64
}

func NewTriangle(amplitude, frequency float64, samplingRate int, symmetry float64) (Oscillator, error) {
	if err := validate(amplitude, frequency, samplingRate); err != nil {
		return nil, err
	}
	// Unlike pulseWidth, 0 and 1 are allowed: they give a sawtooth, not a constant.
	if math.IsNaN(symmetry) || symmetry < 0 || 1 < symmetry {
		return nil, fmt.Errorf("symmetry must be between 0 and 1, got: %v", symmetry)
	}

	peak := 2 * math.Pi * symmetry
	return &triangle{
		phaseTracker: newPhaseTracker(frequency, samplingRate),
		amplitude:    amplitude,
		symmetry:     symmetry,
		peak:         peak,
		// At symmetry 0 or 1 one part has zero length, so its slope is infinite (or NaN at zero amplitude).
		// next() never uses it, because the phase is never below a peak of 0 and never reaches a peak of 2PI.
		riseSlope: 2 * amplitude / peak,
		fallSlope: 2 * amplitude / (2*math.Pi - peak),
	}, nil
}

func (t *triangle) next() float64 {
	phase := t.advance()
	if phase < t.peak {
		return t.riseSlope*phase - t.amplitude
	}
	return t.amplitude - t.fallSlope*(phase-t.peak)
}

func (t *triangle) Read(p []byte) (int, error) {
	return encode(p, t.next)
}

func (t *triangle) String() string {
	return describe("triangle", t.amplitude, t.phaseTracker, fmt.Sprintf("symmetry %v", t.symmetry))
}
