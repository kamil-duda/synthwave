package oscillator

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTriangle_Next(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
		symmetry     float64
	}{
		{"symmetry 0.5 is a symmetric triangle", 1, 100, 4_410, 0.5},
		{"symmetry 0.2 rises fast and falls slowly", 1, 100, 4_410, 0.2},
		{"symmetry 0.8 rises slowly and falls fast", 1, 100, 4_410, 0.8},
		{"half amplitude", 0.5, 100, 4_410, 0.25},
		{"zero symmetry is a falling sawtooth", 1, 100, 4_410, 0},
		{"full symmetry is a rising sawtooth", 1, 100, 4_410, 1},
		{"zero amplitude at zero symmetry", 0, 100, 4_410, 0},
		{"zero amplitude at full symmetry", 0, 100, 4_410, 1},
		{"440 Hz at 44.1 kHz", 1, 440, 44_100, 0.3},
		{"just below Nyquist", 1, 22_049, 44_100, 0.5},
		{"fractional 100.5 Hz", 1, 100.5, 4_410, 0.2},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewTriangle(tt.amplitude, tt.frequency, tt.samplingRate, tt.symmetry)
			require.NoError(t, err)

			// three full periods, so the phase wraps around 2PI at least twice
			samples := int(3 * float64(tt.samplingRate) / tt.frequency)
			for i := range samples {
				// Sample i lies x = (i*f mod samplingRate) / samplingRate of the way into its period.
				// The wave rises from -amplitude to +amplitude while x < symmetry and falls back for the rest of the period.
				// The position is exact, because every frequency here is a whole or half number of Hz, so i*f has no rounding error.
				// At symmetry 0 and 1 the wave jumps at the end of the period. No case puts a sample closer than 2 to it, except i = 0.
				x := math.Mod(float64(i)*tt.frequency, float64(tt.samplingRate)) / float64(tt.samplingRate)
				expected := tt.amplitude * (1 - 2*(x-tt.symmetry)/(1-tt.symmetry))
				if x < tt.symmetry {
					expected = tt.amplitude * (-1 + 2*x/tt.symmetry)
				}
				assert.InDelta(t, expected, osc.next(), 1e-9, "sample %d", i)
			}
		})
	}
}

func TestNewTriangle_validation(t *testing.T) {
	const (
		amplitudeError = "amplitude must be between 0 and 1"
		symmetryError  = "symmetry must be between 0 and 1"
	)
	tests := []struct {
		name          string
		amplitude     float64
		symmetry      float64
		expectedError string
	}{
		{"negative symmetry -0.1", 1, -0.1, symmetryError},
		{"too large symmetry 1.1", 1, 1.1, symmetryError},
		{"negative infinite symmetry", 1, math.Inf(-1), symmetryError},
		{"infinite symmetry", 1, math.Inf(1), symmetryError},
		{"NaN symmetry", 1, math.NaN(), symmetryError},
		{"amplitude is checked before symmetry", 2, -0.1, amplitudeError},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewTriangle(tt.amplitude, 440, 44_100, tt.symmetry)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func TestNewTriangle_boundaries(t *testing.T) {
	tests := []struct {
		name     string
		symmetry float64
	}{
		{"zero symmetry is a falling sawtooth", 0},
		{"full symmetry is a rising sawtooth", 1},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewTriangle(1, 440, 44_100, tt.symmetry)
			assert.NoError(t, err)
			assert.NotNil(t, osc)
		})
	}
}

func TestTriangle_String(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
		symmetry     float64
		expected     string
	}{
		{"main.go settings", 0.2, 440, 44_100, 0.2, "triangle: 440 Hz, amplitude 0.2, symmetry 0.2, sampling rate 44100 Hz"},
		{"fractional frequency", 0.5, 261.63, 48_000, 0.3, "triangle: 261.63 Hz, amplitude 0.5, symmetry 0.3, sampling rate 48000 Hz"},
		{"zero symmetry is a falling sawtooth", 1, 100.5, 4_410, 0, "triangle: 100.5 Hz, amplitude 1, symmetry 0, sampling rate 4410 Hz"},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewTriangle(tt.amplitude, tt.frequency, tt.samplingRate, tt.symmetry)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, osc.String())
		})
	}
}

func BenchmarkTriangle_Next(b *testing.B) {
	osc, err := NewTriangle(1, 440, 44_100, 0.2)
	require.NoError(b, err)

	for b.Loop() {
		osc.next()
	}
}

func BenchmarkTriangle_Read(b *testing.B) {
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
			osc, err := NewTriangle(1, bm.frequency, 44_100, 0.2)
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
