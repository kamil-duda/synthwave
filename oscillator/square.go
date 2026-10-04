package oscillator

import "math"

// square generates a naive square wave: +amplitude in the first half of every period and -amplitude in the second.
// An ideal square wave is a sum of infinitely many odd harmonics (f, 3f, 5f, ...) with amplitudes falling as 1/n.
// Sampling cannot represent the harmonics above the Nyquist frequency, so they alias: each one folds back below it as a tone
// that is not a multiple of f (at 44.1 kHz the 101st harmonic of 440 Hz, 44440 Hz, is heard at 340 Hz with 1/101 of the amplitude).
// Low notes alias quietly, because only weak, high harmonics fold back. High notes fold back strong ones and sound out of tune.
// A band-limited square (e.g. PolyBLEP) avoids this by rounding off each jump over a couple of samples,
// which removes most of the harmonics above the Nyquist frequency.
type square struct {
	phaseTracker
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
}

func NewSquare(amplitude, frequency float64, samplingRate int) (Oscillator, error) {
	if err := validate(amplitude, frequency, samplingRate); err != nil {
		return nil, err
	}
	return &square{
		phaseTracker: newPhaseTracker(frequency, samplingRate),
		amplitude:    amplitude,
	}, nil
}

func (s *square) next() float64 {
	if s.advance() < math.Pi {
		return s.amplitude
	}
	return -s.amplitude
}

func (s *square) Read(p []byte) (int, error) {
	return encode(p, s.next)
}

func (s *square) String() string {
	return describe("square", s.amplitude, s.phaseTracker)
}
