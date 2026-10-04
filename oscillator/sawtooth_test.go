package oscillator

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSawtooth_Next(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
	}{
		{"full amplitude 100 Hz", 1, 100, 4_410},
		{"half amplitude 100 Hz", 0.5, 100, 4_410},
		{"zero amplitude", 0, 100, 4_410},
		{"440 Hz at 44.1 kHz", 1, 440, 44_100},
		{"just below Nyquist", 1, 22_049, 44_100},
		{"fractional 100.5 Hz", 1, 100.5, 4_410},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSawtooth(tt.amplitude, tt.frequency, tt.samplingRate)
			require.NoError(t, err)

			// three full periods, so the phase wraps around 2PI at least twice
			samples := int(3 * float64(tt.samplingRate) / tt.frequency)
			for i := range samples {
				// Sample i lies x = (i*f mod samplingRate) / samplingRate of the way into its period,
				// where the wave has risen from -amplitude by 2*amplitude*x.
				// The position is exact, because every frequency here is a whole or half number of Hz, so i*f has no rounding error.
				// No case puts a sample closer than 2 to the jump at the end of the period, except i = 0.
				x := math.Mod(float64(i)*tt.frequency, float64(tt.samplingRate)) / float64(tt.samplingRate)
				expected := tt.amplitude * (2*x - 1)
				assert.InDelta(t, expected, osc.next(), 1e-9, "sample %d", i)
			}
		})
	}
}

func TestSawtooth_String(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
		expected     string
	}{
		{"main.go settings", 0.2, 440, 44_100, "sawtooth: 440 Hz, amplitude 0.2, sampling rate 44100 Hz"},
		{"fractional frequency", 0.5, 261.63, 48_000, "sawtooth: 261.63 Hz, amplitude 0.5, sampling rate 48000 Hz"},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSawtooth(tt.amplitude, tt.frequency, tt.samplingRate)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, osc.String())
		})
	}
}

func BenchmarkSawtooth_Next(b *testing.B) {
	osc, err := NewSawtooth(1, 440, 44_100)
	require.NoError(b, err)

	for b.Loop() {
		osc.next()
	}
}

func BenchmarkSawtooth_Read(b *testing.B) {
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
			osc, err := NewSawtooth(1, bm.frequency, 44_100)
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
