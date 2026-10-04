package oscillator

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSquare_Next(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
	}{
		{"full amplitude 100 Hz", 1, 100, 4_410},
		{"half amplitude 100 Hz", 0.5, 100, 4_410},
		{"440 Hz at 44.1 kHz", 1, 440, 44_100},
		{"just below Nyquist", 1, 22_049, 44_100},
		{"fractional 100.5 Hz", 1, 100.5, 4_410},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSquare(tt.amplitude, tt.frequency, tt.samplingRate)
			require.NoError(t, err)

			// three full periods, so the phase wraps around 2PI at least twice
			samples := int(3 * float64(tt.samplingRate) / tt.frequency)
			for i := range samples {
				// Sample i lies (i*f mod samplingRate) / samplingRate of the way into its period and the first half is positive.
				// The position is exact, because every frequency here is a whole or half number of Hz, so i*f has no rounding error.
				// No case puts a sample closer than 1 to an edge, except i = 0.
				expected := tt.amplitude
				if 2*math.Mod(float64(i)*tt.frequency, float64(tt.samplingRate)) >= float64(tt.samplingRate) {
					expected = -tt.amplitude
				}
				assert.Equal(t, expected, osc.next(), "sample %d", i)
			}
		})
	}
}

func TestSquare_String(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
		expected     string
	}{
		{"main.go settings", 0.2, 440, 44_100, "square: 440 Hz, amplitude 0.2, sampling rate 44100 Hz"},
		{"fractional frequency", 0.5, 261.63, 48_000, "square: 261.63 Hz, amplitude 0.5, sampling rate 48000 Hz"},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSquare(tt.amplitude, tt.frequency, tt.samplingRate)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, osc.String())
		})
	}
}

func BenchmarkSquare_Next(b *testing.B) {
	osc, err := NewSquare(1, 440, 44_100)
	require.NoError(b, err)

	for b.Loop() {
		osc.next()
	}
}

func BenchmarkSquare_Read(b *testing.B) {
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
			osc, err := NewSquare(1, bm.frequency, 44_100)
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
