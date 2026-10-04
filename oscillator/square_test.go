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

func TestNewSquare_validation(t *testing.T) {
	const (
		amplitudeError    = "amplitude must be between 0 and 1"
		frequencyError    = "frequency must be positive"
		samplingRateError = "samplingRate must be positive"
		nyquistError      = "frequency must be below the Nyquist frequency"
	)
	tests := []struct {
		name          string
		amplitude     float64
		frequency     uint
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
			_, err := NewSquare(tt.amplitude, tt.frequency, tt.samplingRate)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func TestNewSquare_boundaries(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    uint
		samplingRate int
	}{
		{"zero amplitude", 0, 440, 44_100},
		{"full amplitude", 1, 440, 44_100},
		{"frequency just below Nyquist", 1, 22_049, 44_100},
		{"frequency just below Nyquist with odd sampling rate", 1, 2_200, 4_401},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSquare(tt.amplitude, tt.frequency, tt.samplingRate)
			assert.NoError(t, err)
			assert.NotNil(t, osc)
		})
	}
}

func TestSquare_Next(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    uint
		samplingRate int
	}{
		{"full amplitude 100 Hz", 1, 100, 4_410},
		{"half amplitude 100 Hz", 0.5, 100, 4_410},
		{"440 Hz at 44.1 kHz", 1, 440, 44_100},
		{"just below Nyquist", 1, 22_049, 44_100},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSquare(tt.amplitude, tt.frequency, tt.samplingRate)
			require.NoError(t, err)

			// three full periods, so the phase wraps around 2PI at least twice
			samples := 3 * tt.samplingRate / int(tt.frequency)
			for i := range samples {
				// Sample i lies (i*f mod samplingRate) / samplingRate of the way into its period and the first half is positive.
				// Integer math keeps the expected sign exact. No case puts a sample exactly on an edge, except i = 0.
				expected := tt.amplitude
				if 2*(i*int(tt.frequency)%tt.samplingRate) >= tt.samplingRate {
					expected = -tt.amplitude
				}
				assert.Equal(t, expected, osc.next(), "sample %d", i)
			}
		})
	}
}

func TestSquare_NextSignedInt16(t *testing.T) {
	tests := []struct {
		name        string
		amplitude   float64
		sampleIndex int
		expected    int16
	}{
		{"positive peak at start", 1, 0, 32_767},
		{"last positive sample of the first period", 1, 22, 32_767},
		{"first negative sample", 1, 23, -32_767},
		{"last negative sample of the first period", 1, 44, -32_767},
		{"positive again after the phase wraps", 1, 45, 32_767},
		{"half amplitude rounds half away from zero", 0.5, 0, 16_384},
		{"negative half amplitude rounds half away from zero", 0.5, 23, -16_384},
		{"quarter amplitude rounds up", 0.25, 0, 8_192},
		{"negative quarter amplitude rounds down", 0.25, 23, -8_192},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 100 Hz at 4410 Hz is 44.1 samples per period: samples 0-22 are positive, 23-44 negative, 45 starts the next period
			osc, err := NewSquare(tt.amplitude, 100, 4_410)
			require.NoError(t, err)

			for range tt.sampleIndex {
				osc.nextSignedInt16()
			}
			assert.Equal(t, tt.expected, osc.nextSignedInt16())
		})
	}
}

func TestSquare_Read(t *testing.T) {
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
			osc, err := NewSquare(1, 440, 44_100)
			require.NoError(t, err)
			twin, err := NewSquare(1, 440, 44_100)
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

func TestSquare_Read_encoding(t *testing.T) {
	tests := []struct {
		name          string
		amplitude     float64
		sampleIndex   int
		expectedBytes []byte
	}{
		{"positive peak 32767 = 0x7FFF", 1, 0, []byte{0xFF, 0x7F}},
		{"negative peak -32767 = 0x8001 in two's complement", 1, 23, []byte{0x01, 0x80}},
		{"quarter amplitude 8192 = 0x2000", 0.25, 0, []byte{0x00, 0x20}},
		{"negative quarter amplitude -8192 = 0xE000 in two's complement", 0.25, 23, []byte{0x00, 0xE0}},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 100 Hz at 4410 Hz: samples 0-22 are positive, 23-44 negative
			osc, err := NewSquare(tt.amplitude, 100, 4_410)
			require.NoError(t, err)

			p := make([]byte, 2*(tt.sampleIndex+1))
			_, err = osc.Read(p)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBytes, p[len(p)-2:])
		})
	}
}

func TestSquare_Read_continuity(t *testing.T) {
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
			whole, err := NewSquare(1, 440, 44_100)
			require.NoError(t, err)
			chunked, err := NewSquare(1, 440, 44_100)
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

func TestSquare_Read_shortBuffer(t *testing.T) {
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
			osc, err := NewSquare(1, 440, 44_100)
			require.NoError(t, err)

			n, err := osc.Read(make([]byte, tt.bufferLength))
			assert.Equal(t, 0, n)
			assert.ErrorIs(t, err, io.ErrShortBuffer)
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

func BenchmarkSquare_NextSignedInt16(b *testing.B) {
	osc, err := NewSquare(1, 440, 44_100)
	require.NoError(b, err)

	for b.Loop() {
		osc.nextSignedInt16()
	}
}

func BenchmarkSquare_Read(b *testing.B) {
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
			osc, err := NewSquare(1, 440, 44_100)
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
