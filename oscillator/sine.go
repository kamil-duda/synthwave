package oscillator

import "math"

// sine generates sinusoidal waveforms for audio synthesis.
// It maintains phase information to produce continuous sine waves at a specified frequency.
type sine struct {
	phaseTracker
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
}

func NewSine(amplitude, frequency float64, samplingRate int) (Oscillator, error) {
	if err := validate(amplitude, frequency, samplingRate); err != nil {
		return nil, err
	}
	return &sine{
		phaseTracker: newPhaseTracker(frequency, samplingRate),
		amplitude:    amplitude,
	}, nil
}

func (s *sine) next() float64 {
	return s.amplitude * math.Sin(s.advance())
}

func (s *sine) Read(p []byte) (int, error) {
	return encode(p, s.next)
}

func (s *sine) String() string {
	return describe("sine", s.amplitude, s.phaseTracker)
}
