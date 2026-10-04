package oscillator

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
)

// Oscillator generates audio samples. Read encodes them for oto (see encode).
type Oscillator interface {
	io.Reader
	fmt.Stringer
	// next returns the sample at the current phase, in [-amplitude, amplitude], and advances the phase
	next() float64
}

// validate checks the arguments every oscillator takes, in this order: amplitude, frequency, samplingRate and the Nyquist frequency.
// The order matters, because checks mask each other: a negative samplingRate would also fail the Nyquist check.
func validate(amplitude, frequency float64, samplingRate int) error {
	if math.IsNaN(amplitude) || amplitude < 0 || 1 < amplitude {
		return fmt.Errorf("amplitude must be between 0 and 1, got: %v", amplitude)
	}
	if math.IsNaN(frequency) || frequency <= 0 {
		return fmt.Errorf("frequency must be positive, got %v", frequency)
	}
	if samplingRate <= 0 {
		return fmt.Errorf("samplingRate must be positive, got: %v", samplingRate)
	}
	// The Nyquist frequency (half the sampling rate) is the highest frequency that sampling can represent.
	// Each period needs at least two samples. Higher frequencies alias, i.e., their samples are identical to those of a lower frequency
	// (30 kHz sampled at 44.1 kHz sounds like 14.1 kHz), and at exactly the Nyquist frequency a sine starting at phase 0 is sampled only at its zero crossings (silence).
	// The check covers only the fundamental frequency f: the harmonics of the other waveforms above it alias anyway (see square).
	// Staying below it also keeps phaseStep < PI, so advance() never needs to wrap the phase more than once.
	// The division is done on floats, because integer division would truncate an odd sampling rate (4401 / 2 = 2200 instead of 2200.5) and wrongly reject a valid 2200 Hz.
	nyquistFrequency := float64(samplingRate) / 2
	if frequency >= nyquistFrequency {
		return fmt.Errorf("frequency must be below the Nyquist frequency %v Hz, got: %v", nyquistFrequency, frequency)
	}
	return nil
}

// angularFrequency converts frequency f [periods/second] into angular frequency [rad/second].
// One full period (360 deg) is 2 PI rad.
// Having 1Hz means one full period per second, so an angular frequency of 2 PI rad / second.
// Higher frequency means more angular frequency to keep up.
func angularFrequency(f float64) float64 {
	return 2 * math.Pi * f
}

// phaseTracker tracks where in its period an oscillator is: the phase goes from 0 to 2PI once per period.
type phaseTracker struct {
	// phase is the current phase angle in radians, in [0, 2PI)
	phase float64
	// phaseStep is the phase increment per one sample, calculated as angular frequency / sample rate
	phaseStep float64
	// frequency [Hz] and samplingRate [samples/second] are what phaseStep was calculated from, kept for describe
	frequency    float64
	samplingRate int
}

func newPhaseTracker(frequency float64, samplingRate int) phaseTracker {
	return phaseTracker{
		phase:        0,
		phaseStep:    angularFrequency(frequency) / float64(samplingRate),
		frequency:    frequency,
		samplingRate: samplingRate,
	}
}

// advance returns the current phase and moves it on by phaseStep.
// A single subtraction is enough to wrap it at 2PI, because the Nyquist check in validate keeps phaseStep < PI.
func (a *phaseTracker) advance() float64 {
	phase := a.phase
	a.phase += a.phaseStep
	if a.phase >= 2*math.Pi {
		a.phase -= 2 * math.Pi
	}
	return phase
}

// toSignedInt16 scales a sample in [-1, 1] to int16. math.Round keeps it in [-32767, 32767], symmetric around 0.
func toSignedInt16(sample float64) int16 {
	return int16(math.Round(sample * math.MaxInt16))
}

// encode fills p with samples from next in the format main.go sets for oto: one little-endian int16 (2 bytes) per sample, mono.
// It returns io.ErrShortBuffer if p cannot hold a single sample and leaves a trailing odd byte untouched.
// Every oscillator's Read calls it with its own next. A call through a function value cannot be inlined,
// which costs sine a few percent and the other oscillators nothing measurable.
func encode(p []byte, next func() float64) (int, error) {
	if len(p) < 2 {
		return 0, io.ErrShortBuffer
	}
	n := 0
	for ; n+1 < len(p); n += 2 {
		// converting int16 to uint16 does not change the internal binary representation
		binary.LittleEndian.PutUint16(p[n:], uint16(toSignedInt16(next())))
	}
	return n, nil
}

// describe formats what every oscillator's String shows, e.g. "pulse: 440 Hz, amplitude 0.2, pulseWidth 0.2, sampling rate 44100 Hz".
// parameters are the oscillator's own settings, already formatted; they go between the amplitude and the sampling rate.
func describe(name string, amplitude float64, t phaseTracker, parameters ...string) string {
	parts := append([]string{fmt.Sprintf("%v Hz", t.frequency), fmt.Sprintf("amplitude %v", amplitude)}, parameters...)
	parts = append(parts, fmt.Sprintf("sampling rate %v Hz", t.samplingRate))
	return name + ": " + strings.Join(parts, ", ")
}
