package oscillator

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPulse_Next(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
		pulseWidth   float64
	}{
		{"pulse width 0.2", 1, 100, 4_410, 0.2},
		{"pulse width 0.25 at half amplitude", 0.5, 100, 4_410, 0.25},
		{"pulse width 0.5 is a square", 1, 100, 4_410, 0.5},
		{"pulse width 0.8", 1, 100, 4_410, 0.8},
		{"440 Hz at 44.1 kHz", 1, 440, 44_100, 0.25},
		{"just below Nyquist", 1, 22_049, 44_100, 0.25},
		{"fractional 100.5 Hz", 1, 100.5, 4_410, 0.2},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewPulse(tt.amplitude, tt.frequency, tt.samplingRate, tt.pulseWidth)
			require.NoError(t, err)

			// three full periods, so the phase wraps around 2PI at least twice
			samples := int(3 * float64(tt.samplingRate) / tt.frequency)
			for i := range samples {
				// Sample i lies (i*f mod samplingRate) / samplingRate of the way into its period and is high while that is below pulseWidth.
				// The position is exact, because every frequency here is a whole or half number of Hz, so i*f has no rounding error.
				// No case puts it closer than 2 to pulseWidth * samplingRate, so no sample sits on an edge.
				expected := tt.amplitude
				if math.Mod(float64(i)*tt.frequency, float64(tt.samplingRate)) >= tt.pulseWidth*float64(tt.samplingRate) {
					expected = -tt.amplitude
				}
				assert.Equal(t, expected, osc.next(), "sample %d", i)
			}
		})
	}
}

func TestNewPulse_validation(t *testing.T) {
	const (
		amplitudeError  = "amplitude must be between 0 and 1"
		pulseWidthError = "pulseWidth must be greater than 0 and less than 1"
	)
	tests := []struct {
		name          string
		amplitude     float64
		pulseWidth    float64
		expectedError string
	}{
		{"zero pulse width", 1, 0, pulseWidthError},
		{"full pulse width", 1, 1, pulseWidthError},
		{"negative pulse width", 1, -0.2, pulseWidthError},
		{"too large pulse width", 1, 1.2, pulseWidthError},
		{"infinite pulse width", 1, math.Inf(1), pulseWidthError},
		{"NaN pulse width", 1, math.NaN(), pulseWidthError},
		{"amplitude is checked before pulseWidth", 2, 0, amplitudeError},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewPulse(tt.amplitude, 440, 44_100, tt.pulseWidth)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func TestNewPulse_boundaries(t *testing.T) {
	tests := []struct {
		name       string
		pulseWidth float64
	}{
		{"narrow pulse width 0.01", 0.01},
		{"wide pulse width 0.99", 0.99},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewPulse(1, 440, 44_100, tt.pulseWidth)
			assert.NoError(t, err)
			assert.NotNil(t, osc)
		})
	}
}

func TestPulse_String(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
		pulseWidth   float64
		expected     string
	}{
		{"main.go settings", 0.2, 440, 44_100, 0.2, "pulse: 440 Hz, amplitude 0.2, pulseWidth 0.2, sampling rate 44100 Hz"},
		{"fractional frequency", 0.5, 261.63, 48_000, 0.25, "pulse: 261.63 Hz, amplitude 0.5, pulseWidth 0.25, sampling rate 48000 Hz"},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewPulse(tt.amplitude, tt.frequency, tt.samplingRate, tt.pulseWidth)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, osc.String())
		})
	}
}

func BenchmarkPulse_Next(b *testing.B) {
	osc, err := NewPulse(1, 440, 44_100, 0.2)
	require.NoError(b, err)

	for b.Loop() {
		osc.next()
	}
}

func BenchmarkPulse_Read(b *testing.B) {
	// higher frequencies are slower: the branches in next() change direction more often and the CPU predicts them worse
	benchmarks := []struct {
		name      string
		frequency float64
	}{
		{"A4 440 Hz", 440},
		{"C8 4186 Hz", 4_186},
		{"just below Nyquist", 22_049},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			osc, err := NewPulse(1, bm.frequency, 44_100, 0.2)
			require.NoError(b, err)
			// 4096 bytes is what oto requests per Read
			p := make([]byte, 4_096)

			b.SetBytes(int64(len(p)))
			for b.Loop() {
				if _, err := osc.Read(p); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
