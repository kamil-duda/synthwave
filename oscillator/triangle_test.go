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

func TestNewTriangle_validation(t *testing.T) {
	const (
		amplitudeError    = "amplitude must be between 0 and 1"
		frequencyError    = "frequency must be positive"
		samplingRateError = "samplingRate must be positive"
		nyquistError      = "frequency must be below the Nyquist frequency"
		symmetryError     = "symmetry must be between 0 and 1"
	)
	tests := []struct {
		name          string
		amplitude     float64
		frequency     float64
		samplingRate  int
		symmetry      float64
		expectedError string
	}{
		{"negative amplitude -0.1", -0.1, 440, 44_100, 0.5, amplitudeError},
		{"negative amplitude -0.01", -0.01, 440, 44_100, 0.5, amplitudeError},
		{"negative amplitude -1", -1, 440, 44_100, 0.5, amplitudeError},
		{"negative amplitude -10", -10, 440, 44_100, 0.5, amplitudeError},
		{"too large amplitude 1.01", 1.01, 440, 44_100, 0.5, amplitudeError},
		{"too large amplitude 10", 10, 440, 44_100, 0.5, amplitudeError},
		{"infinite amplitude", math.Inf(1), 440, 44_100, 0.5, amplitudeError},
		{"NaN amplitude", math.NaN(), 440, 44_100, 0.5, amplitudeError},
		{"zero frequency", 1, 0, 44_100, 0.5, frequencyError},
		{"negative frequency -440", 1, -440, 44_100, 0.5, frequencyError},
		{"negative fractional frequency -0.5", 1, -0.5, 44_100, 0.5, frequencyError},
		{"negative infinite frequency", 1, math.Inf(-1), 44_100, 0.5, frequencyError},
		{"NaN frequency", 1, math.NaN(), 44_100, 0.5, frequencyError},
		{"infinite frequency", 1, math.Inf(1), 44_100, 0.5, nyquistError},
		{"frequency at Nyquist", 1, 22_050, 44_100, 0.5, nyquistError},
		{"frequency above Nyquist", 1, 30_000, 44_100, 0.5, nyquistError},
		{"frequency above sampling rate", 1, 100_000, 44_100, 0.5, nyquistError},
		{"frequency just above Nyquist with odd sampling rate", 1, 2_201, 4_401, 0.5, nyquistError},
		{"zero sampling rate", 1, 440, 0, 0.5, samplingRateError},
		{"negative sampling rate -1", 1, 440, -1, 0.5, samplingRateError},
		{"negative sampling rate -10", 1, 440, -10, 0.5, samplingRateError},
		{"negative symmetry -0.1", 1, 440, 44_100, -0.1, symmetryError},
		{"too large symmetry 1.1", 1, 440, 44_100, 1.1, symmetryError},
		{"negative infinite symmetry", 1, 440, 44_100, math.Inf(-1), symmetryError},
		{"infinite symmetry", 1, 440, 44_100, math.Inf(1), symmetryError},
		{"NaN symmetry", 1, 440, 44_100, math.NaN(), symmetryError},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewTriangle(tt.amplitude, tt.frequency, tt.samplingRate, tt.symmetry)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func TestNewTriangle_boundaries(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    float64
		samplingRate int
		symmetry     float64
	}{
		{"zero amplitude", 0, 440, 44_100, 0.5},
		{"full amplitude", 1, 440, 44_100, 0.5},
		{"fractional frequency below 1 Hz", 1, 0.5, 44_100, 0.5},
		{"frequency just below Nyquist", 1, 22_049, 44_100, 0.5},
		{"frequency just below Nyquist with odd sampling rate", 1, 2_200, 4_401, 0.5},
		{"zero symmetry is a falling sawtooth", 1, 440, 44_100, 0},
		{"full symmetry is a rising sawtooth", 1, 440, 44_100, 1},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewTriangle(tt.amplitude, tt.frequency, tt.samplingRate, tt.symmetry)
			assert.NoError(t, err)
			assert.NotNil(t, osc)
		})
	}
}

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

func TestTriangle_NextSignedInt16(t *testing.T) {
	tests := []struct {
		name        string
		amplitude   float64
		symmetry    float64
		sampleIndex int
		expected    int16
	}{
		{"negative peak at start", 1, 0.2, 0, -32_767},
		{"rising rounds toward zero", 1, 0.2, 4, -3_046},
		{"last rising sample rounds down", 1, 0.2, 8, 26_674},
		{"first falling sample rounds up", 1, 0.2, 9, 32_433},
		{"falling rounds down", 1, 0.2, 23, 6_427},
		{"last falling sample of the first period", 1, 0.2, 44, -32_581},
		{"rising again after the phase wraps", 1, 0.2, 45, -26_080},
		{"symmetric last rising sample", 1, 0.5, 22, 32_618},
		{"symmetric first falling sample", 1, 0.5, 23, 29_944},
		{"half amplitude rounds half away from zero", 0.5, 0.2, 0, -16_384},
		{"zero symmetry starts at the positive peak", 1, 0, 0, 32_767},
		{"zero symmetry falls through zero", 1, 0, 23, -1_412},
		{"full symmetry rises through zero", 1, 1, 23, 1_412},
		{"full symmetry last sample before the jump", 1, 1, 44, 32_618},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 100 Hz at 4410 Hz: sample i lies at position 100*i mod 4410 of the 4410 positions in a period.
			// Symmetry 0.2 rises below position 882 (samples 0-8) and falls for the rest of the period (samples 9-44),
			// e.g. sample 9: (1 - 2 * (900 - 882) / (0.8 * 4410)) * 32767 = 32432.64 -> 32433.
			// Symmetry 0.5 peaks at position 2205, between samples 22 and 23.
			osc, err := NewTriangle(tt.amplitude, 100, 4_410, tt.symmetry)
			require.NoError(t, err)

			for range tt.sampleIndex {
				osc.nextSignedInt16()
			}
			assert.Equal(t, tt.expected, osc.nextSignedInt16())
		})
	}
}

func TestTriangle_Read(t *testing.T) {
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
			osc, err := NewTriangle(1, 440, 44_100, 0.2)
			require.NoError(t, err)
			twin, err := NewTriangle(1, 440, 44_100, 0.2)
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

func TestTriangle_Read_encoding(t *testing.T) {
	tests := []struct {
		name          string
		sampleIndex   int
		expectedBytes []byte
	}{
		{"negative peak -32767 = 0x8001 in two's complement", 0, []byte{0x01, 0x80}},
		{"negative -3046 = 0xF41A in two's complement", 4, []byte{0x1A, 0xF4}},
		{"positive 32433 = 0x7EB1", 9, []byte{0xB1, 0x7E}},
		{"positive 6427 = 0x191B", 23, []byte{0x1B, 0x19}},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 100 Hz at 4410 Hz with symmetry 0.2, the same samples as in TestTriangle_NextSignedInt16
			osc, err := NewTriangle(1, 100, 4_410, 0.2)
			require.NoError(t, err)

			p := make([]byte, 2*(tt.sampleIndex+1))
			_, err = osc.Read(p)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBytes, p[len(p)-2:])
		})
	}
}

func TestTriangle_Read_continuity(t *testing.T) {
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
			whole, err := NewTriangle(1, 440, 44_100, 0.2)
			require.NoError(t, err)
			chunked, err := NewTriangle(1, 440, 44_100, 0.2)
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

func TestTriangle_Read_shortBuffer(t *testing.T) {
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
			osc, err := NewTriangle(1, 440, 44_100, 0.2)
			require.NoError(t, err)

			n, err := osc.Read(make([]byte, tt.bufferLength))
			assert.Equal(t, 0, n)
			assert.ErrorIs(t, err, io.ErrShortBuffer)
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

func BenchmarkTriangle_NextSignedInt16(b *testing.B) {
	osc, err := NewTriangle(1, 440, 44_100, 0.2)
	require.NoError(b, err)

	for b.Loop() {
		osc.nextSignedInt16()
	}
}

func BenchmarkTriangle_Read(b *testing.B) {
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
			osc, err := NewTriangle(1, 440, 44_100, 0.2)
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
