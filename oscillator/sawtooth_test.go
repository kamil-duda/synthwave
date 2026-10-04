package oscillator

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSawtooth_validation(t *testing.T) {
	const (
		amplitudeError    = "amplitude must be between 0 and 1"
		frequencyError    = "frequency must be positive"
		samplingRateError = "samplingRate must be positive"
		nyquistError      = "frequency must be below the Nyquist frequency"
	)
	tests := []struct {
		name          string
		amplitude     float64
		frequency     float64
		samplingRate  int
		expectedError string
	}{
		{"negative amplitude -0.1", -0.1, 440, 44_100, amplitudeError},
		{"negative amplitude -0.01", -0.01, 440, 44_100, amplitudeError},
		{"negative amplitude -1", -1, 440, 44_100, amplitudeError},
		{"negative amplitude -10", -10, 440, 44_100, amplitudeError},
		{"too large amplitude 1.01", 1.01, 440, 44_100, amplitudeError},
		{"too large amplitude 10", 10, 440, 44_100, amplitudeError},
		{"infinite amplitude", math.Inf(1), 440, 44_100, amplitudeError},
		{"NaN amplitude", math.NaN(), 440, 44_100, amplitudeError},
		{"zero frequency", 1, 0, 44_100, frequencyError},
		{"negative frequency -440", 1, -440, 44_100, frequencyError},
		{"negative fractional frequency -0.5", 1, -0.5, 44_100, frequencyError},
		{"negative infinite frequency", 1, math.Inf(-1), 44_100, frequencyError},
		{"NaN frequency", 1, math.NaN(), 44_100, frequencyError},
		{"infinite frequency", 1, math.Inf(1), 44_100, nyquistError},
		{"frequency at Nyquist", 1, 22_050, 44_100, nyquistError},
		{"frequency above Nyquist", 1, 30_000, 44_100, nyquistError},
		{"frequency above sampling rate", 1, 100_000, 44_100, nyquistError},
		{"frequency just above Nyquist with odd sampling rate", 1, 2_201, 4_401, nyquistError},
		{"zero sampling rate", 1, 440, 0, samplingRateError},
		{"negative sampling rate -1", 1, 440, -1, samplingRateError},
		{"negative sampling rate -10", 1, 440, -10, samplingRateError},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewSawtooth(tt.amplitude, tt.frequency, tt.samplingRate)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func TestNewSawtooth_boundaries(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
	}{
		{"zero amplitude", 0, 440, 44_100},
		{"full amplitude", 1, 440, 44_100},
		{"fractional frequency below 1 Hz", 1, 0.5, 44_100},
		{"frequency just below Nyquist", 1, 22_049, 44_100},
		{"frequency just below Nyquist with odd sampling rate", 1, 2_200, 4_401},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSawtooth(tt.amplitude, tt.frequency, tt.samplingRate)
			assert.NoError(t, err)
			assert.NotNil(t, osc)
		})
	}
}

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

func TestSawtooth_NextSignedInt16(t *testing.T) {
	tests := []struct {
		name        string
		amplitude   float64
		sampleIndex int
		expected    int16
	}{
		{"negative peak at start", 1, 0, -32_767},
		{"negative rounds away from zero", 1, 1, -31_281},
		{"negative rounds toward zero", 1, 22, -74},
		{"rounds up", 1, 23, 1_412},
		{"rounds down", 1, 33, 16_272},
		{"last sample before the jump", 1, 44, 32_618},
		{"low again after the phase wraps", 1, 45, -31_430},
		{"half amplitude rounds half away from zero", 0.5, 0, -16_384},
		{"quarter amplitude negative peak", 0.25, 0, -8_192},
		{"quarter amplitude last sample before the jump", 0.25, 44, 8_155},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 100 Hz at 4410 Hz: sample i lies x = (100*i mod 4410) / 4410 of the way into its period and is amplitude * (2x - 1).
			// E.g. sample 23: (2 * 2300/4410 - 1) * 32767 = 1411.73 -> 1412. Sample 44 is the last one before the jump,
			// 45 (x = 90/4410) starts the next period.
			osc, err := NewSawtooth(tt.amplitude, 100, 4_410)
			require.NoError(t, err)

			for range tt.sampleIndex {
				osc.nextSignedInt16()
			}
			assert.Equal(t, tt.expected, osc.nextSignedInt16())
		})
	}
}

func TestSawtooth_Read(t *testing.T) {
	tests := []struct {
		name         string
		bufferLength int
		expectedN    int
	}{
		{"one sample", 2, 2},
		{"several samples", 8, 8},
		{"odd length leaves last byte untouched", 7, 6},
		{"half a second at 44.1 kHz", 44_100, 44_100},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSawtooth(1, 440, 44_100)
			require.NoError(t, err)
			twin, err := NewSawtooth(1, 440, 44_100)
			require.NoError(t, err)

			p := bytes.Repeat([]byte{0xAA}, tt.bufferLength)
			n, err := osc.Read(p)
			require.NoError(t, err)
			require.Equal(t, tt.expectedN, n)

			expected := make([]int16, n/2)
			actual := make([]int16, n/2)
			for i := range expected {
				expected[i] = twin.nextSignedInt16()
				actual[i] = int16(binary.LittleEndian.Uint16(p[2*i:]))
			}
			assert.Equal(t, expected, actual)
			assert.Equal(t, bytes.Repeat([]byte{0xAA}, len(p)-n), p[n:])
		})
	}
}

func TestSawtooth_Read_encoding(t *testing.T) {
	tests := []struct {
		name          string
		sampleIndex   int
		expectedBytes []byte
	}{
		{"negative peak -32767 = 0x8001 in two's complement", 0, []byte{0x01, 0x80}},
		{"negative -74 = 0xFFB6 in two's complement", 22, []byte{0xB6, 0xFF}},
		{"positive 1412 = 0x0584", 23, []byte{0x84, 0x05}},
		{"positive 32618 = 0x7F6A", 44, []byte{0x6A, 0x7F}},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 100 Hz at 4410 Hz, the same samples as in TestSawtooth_NextSignedInt16
			osc, err := NewSawtooth(1, 100, 4_410)
			require.NoError(t, err)

			p := make([]byte, 2*(tt.sampleIndex+1))
			_, err = osc.Read(p)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBytes, p[len(p)-2:])
		})
	}
}

func TestSawtooth_Read_continuity(t *testing.T) {
	tests := []struct {
		name        string
		chunkLength int
	}{
		{"one sample per read", 2},
		{"three samples per read", 6},
		{"two hundred samples per read", 400},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			whole, err := NewSawtooth(1, 440, 44_100)
			require.NoError(t, err)
			chunked, err := NewSawtooth(1, 440, 44_100)
			require.NoError(t, err)

			expected := make([]byte, 1_200)
			_, err = whole.Read(expected)
			require.NoError(t, err)

			actual := make([]byte, 0, len(expected))
			chunk := make([]byte, tt.chunkLength)
			for len(actual) < len(expected) {
				n, err := chunked.Read(chunk)
				require.NoError(t, err)
				require.NotZero(t, n, "Read must make progress")
				actual = append(actual, chunk[:n]...)
			}
			assert.Equal(t, expected, actual)
		})
	}
}

func TestSawtooth_Read_shortBuffer(t *testing.T) {
	tests := []struct {
		name         string
		bufferLength int
	}{
		{"empty buffer", 0},
		{"one byte buffer", 1},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSawtooth(1, 440, 44_100)
			require.NoError(t, err)

			n, err := osc.Read(make([]byte, tt.bufferLength))
			assert.Equal(t, 0, n)
			assert.ErrorIs(t, err, io.ErrShortBuffer)
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

func BenchmarkSawtooth_NextSignedInt16(b *testing.B) {
	osc, err := NewSawtooth(1, 440, 44_100)
	require.NoError(b, err)

	for b.Loop() {
		osc.nextSignedInt16()
	}
}

func BenchmarkSawtooth_Read(b *testing.B) {
	benchmarks := []struct {
		name         string
		bufferLength int
	}{
		{"512 B", 512},
		{"4 KiB", 4_096},
		{"half a second at 44.1 kHz", 44_100},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			osc, err := NewSawtooth(1, 440, 44_100)
			require.NoError(b, err)
			p := make([]byte, bm.bufferLength)

			b.SetBytes(int64(bm.bufferLength))
			for b.Loop() {
				if _, err := osc.Read(p); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
