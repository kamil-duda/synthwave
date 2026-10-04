package oscillator

import "math"

// sawtooth generates a naive rising sawtooth wave: every period it rises linearly from -amplitude towards +amplitude
// and then jumps back down. At 10 samples per period it gives -1, -0.8, ..., 0.6, 0.8, -1, -0.8, ...
// It is the cheapest waveform here: next() is one multiplication and one subtraction, without math.Sin or a branch
// (apart from the phase wrap that every oscillator does).
// A sawtooth contains every harmonic (f, 2f, 3f, ...) with amplitudes falling as 1/n, which makes it bright and buzzy.
// It aliases more than square, which has only the odd harmonics: each harmonic above the Nyquist frequency folds back below it
// (see square). A band-limited sawtooth (e.g. PolyBLEP) would remove most of that.
// The wave spends as much of the period above 0 as below, so unlike pulse it has no DC offset.
type sawtooth struct {
	phaseTracker
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
	// slope is how much the wave rises per radian of phase: 2*amplitude over one period of 2PI, i.e. amplitude / PI
	slope float64
}

func NewSawtooth(amplitude, frequency float64, samplingRate int) (Oscillator, error) {
	if err := validate(amplitude, frequency, samplingRate); err != nil {
		return nil, err
	}
	return &sawtooth{
		phaseTracker: newPhaseTracker(frequency, samplingRate),
		amplitude:    amplitude,
		slope:        amplitude / math.Pi,
	}, nil
}

func (s *sawtooth) next() float64 {
	return s.slope*s.advance() - s.amplitude
}

func (s *sawtooth) Read(p []byte) (int, error) {
	return encode(p, s.next)
}

func (s *sawtooth) String() string {
	return describe("sawtooth", s.amplitude, s.phaseTracker)
}
