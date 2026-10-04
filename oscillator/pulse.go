package oscillator

import (
	"encoding/binary"
	"fmt"
	"io"
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
	// amplitude is the amplitude of the Oscillator's waveform, between 0 and 1
	amplitude float64
	// phase is the current, internal phase angle in radians
	phase float64
	// phaseStep is the phase increment per one sample, calculated as angular frequency / sample rate
	phaseStep float64
	// edge is the phase in radians where the wave drops from +amplitude to -amplitude, 2PI * pulseWidth
	edge float64
}

func NewPulse(amplitude, frequency float64, samplingRate int, pulseWidth float64) (Oscillator, error) {
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
	// At pulseWidth 0 or 1 the wave would never change level, i.e. it would be a constant: silence.
	if math.IsNaN(pulseWidth) || pulseWidth <= 0 || 1 <= pulseWidth {
		return nil, fmt.Errorf("pulseWidth must be greater than 0 and less than 1, got: %v", pulseWidth)
	}

	return &pulse{
		amplitude: amplitude,
		phase:     0,
		phaseStep: angularFrequency(frequency) / float64(samplingRate),
		edge:      2 * math.Pi * pulseWidth,
	}, nil
}

func (p *pulse) next() float64 {
	value := p.amplitude
	if p.phase >= p.edge {
		value = -p.amplitude
	}
	p.phase += p.phaseStep
	if p.phase >= 2*math.Pi {
		p.phase -= 2 * math.Pi
	}
	return value
}

func (p *pulse) nextSignedInt16() int16 {
	return int16(math.Round(p.next() * math.MaxInt16))
}

func (p *pulse) Read(b []byte) (n int, err error) {
	bufferLength := len(b)
	if bufferLength < 2 {
		return 0, io.ErrShortBuffer
	}

	// while index of next element is smaller than the length e.g. (byteIdx=2, byteIdx+1=3, pLength=3)
	byteIdx := 0
	for ; byteIdx+1 < bufferLength; byteIdx += 2 {
		sample := p.nextSignedInt16()
		// converting int16 to uint16 does not change the internal binary representation
		binary.LittleEndian.PutUint16(b[byteIdx:byteIdx+2], uint16(sample))
	}
	return byteIdx, nil
}
