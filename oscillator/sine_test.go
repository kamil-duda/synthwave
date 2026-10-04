package oscillator

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSine_Next(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
	}{
		{"full amplitude 100 Hz", 1, 100, 4_400},
		{"half amplitude 100 Hz", 0.5, 100, 4_400},
		{"zero amplitude", 0, 100, 4_400},
		{"440 Hz at 44.1 kHz", 1, 440, 44_100},
		{"fractional 261.63 Hz (C4)", 1, 261.63, 44_100},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSine(tt.amplitude, tt.frequency, tt.samplingRate)
			require.NoError(t, err)

			// three full periods, so the phase wraps around 2PI at least twice
			samples := int(3 * float64(tt.samplingRate) / tt.frequency)
			for i := range samples {
				expected := tt.amplitude * math.Sin(2*math.Pi*tt.frequency*float64(i)/float64(tt.samplingRate))
				assert.InDelta(t, expected, osc.next(), 1e-9, "sample %d", i)
			}
		})
	}
}

func TestSine_String(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
		expected     string
	}{
		{"main.go settings", 0.2, 440, 44_100, "sine: 440 Hz, amplitude 0.2, sampling rate 44100 Hz"},
		{"fractional frequency", 0.5, 261.63, 48_000, "sine: 261.63 Hz, amplitude 0.5, sampling rate 48000 Hz"},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSine(tt.amplitude, tt.frequency, tt.samplingRate)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, osc.String())
		})
	}
}

func BenchmarkSine_Next(b *testing.B) {
	osc, err := NewSine(1, 440, 44_100)
	require.NoError(b, err)

	for b.Loop() {
		osc.next()
	}
}

func BenchmarkSine_Read(b *testing.B) {
	// the frequency changes the speed, but for sine without a simple trend, because math.Sin dominates the cost
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
			osc, err := NewSine(1, bm.frequency, 44_100)
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
