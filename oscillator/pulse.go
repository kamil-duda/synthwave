package oscillator

import (
	"fmt"
	"math"
)

// pulse generates a naive pulse wave: +amplitude for the first pulseWidth part of every period and -amplitude for the rest.
// pulseWidth (also called duty cycle) shapes the timbre. Harmonic n has an amplitude proportional to |sin(PI*n*pulseWidth)| / n,
// so a pulse width of 1/k has no k-th, 2k-th, ... harmonics: 0.5 is the square wave (odd harmonics only), 0.2 has no 5th, 10th, ...
// Narrower pulses sound thinner. pulseWidth and 1 - pulseWidth sound the same, one is the other upside down and shifted in time.
// The levels stay at +/-amplitude, so any pulseWidth other than 0.5 gives the wave a DC offset (a non-zero average):
// amplitude * (2*pulseWidth - 1), e.g. -0.6 * amplitude at 0.2. It is inaudible, but it adds up when mixing and clicks when the sound
// starts or stops. Removing it here would push the peak above amplitude, so a DC-blocking filter after the oscillators should do it.
// Like square, this pulse is not band-limited, so its harmonics above the Nyquist frequency alias.
type pulse struct {
	phaseTracker
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
	// pulseWidth is the part of the period the wave is high, kept for String
	pulseWidth float64
	// edge is the phase in radians where the wave drops from +amplitude to -amplitude, 2PI * pulseWidth
	edge float64
}

func NewPulse(amplitude, frequency float64, samplingRate int, pulseWidth float64) (Oscillator, error) {
	if err := validate(amplitude, frequency, samplingRate); err != nil {
		return nil, err
	}
	// At pulseWidth 0 or 1 the wave would never change level, i.e. it would be a constant: silence.
	if math.IsNaN(pulseWidth) || pulseWidth <= 0 || 1 <= pulseWidth {
		return nil, fmt.Errorf("pulseWidth must be greater than 0 and less than 1, got: %v", pulseWidth)
	}

	return &pulse{
		phaseTracker: newPhaseTracker(frequency, samplingRate),
		amplitude:    amplitude,
		pulseWidth:   pulseWidth,
		edge:         2 * math.Pi * pulseWidth,
	}, nil
}

func (p *pulse) next() float64 {
	if p.advance() < p.edge {
		return p.amplitude
	}
	return -p.amplitude
}

func (p *pulse) Read(b []byte) (int, error) {
	return encode(b, p.next)
}

func (p *pulse) String() string {
	return describe("pulse", p.amplitude, p.phaseTracker, fmt.Sprintf("pulseWidth %v", p.pulseWidth))
}
