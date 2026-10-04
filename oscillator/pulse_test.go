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

func TestNewPulse_validation(t *testing.T) {
	const (
		amplitudeError    = "amplitude must be between 0 and 1"
		frequencyError    = "frequency must be positive"
		samplingRateError = "samplingRate must be positive"
		nyquistError      = "frequency must be below the Nyquist frequency"
		pulseWidthError   = "pulseWidth must be greater than 0 and less than 1"
	)
	tests := []struct {
		name          string
		amplitude     float64
		frequency     uint
		samplingRate  int
		pulseWidth    float64
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
		{"frequency at Nyquist", 1, 22_050, 44_100, 0.5, nyquistError},
		{"frequency above Nyquist", 1, 30_000, 44_100, 0.5, nyquistError},
		{"frequency above sampling rate", 1, 100_000, 44_100, 0.5, nyquistError},
		{"frequency just above Nyquist with odd sampling rate", 1, 2_201, 4_401, 0.5, nyquistError},
		{"zero sampling rate", 1, 440, 0, 0.5, samplingRateError},
		{"negative sampling rate -1", 1, 440, -1, 0.5, samplingRateError},
		{"negative sampling rate -10", 1, 440, -10, 0.5, samplingRateError},
		{"zero pulse width", 1, 440, 44_100, 0, pulseWidthError},
		{"full pulse width", 1, 440, 44_100, 1, pulseWidthError},
		{"negative pulse width", 1, 440, 44_100, -0.2, pulseWidthError},
		{"too large pulse width", 1, 440, 44_100, 1.2, pulseWidthError},
		{"infinite pulse width", 1, 440, 44_100, math.Inf(1), pulseWidthError},
		{"NaN pulse width", 1, 440, 44_100, math.NaN(), pulseWidthError},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewPulse(tt.amplitude, tt.frequency, tt.samplingRate, tt.pulseWidth)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func TestNewPulse_boundaries(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    uint
		samplingRate int
		pulseWidth   float64
	}{
		{"zero amplitude", 0, 440, 44_100, 0.5},
		{"full amplitude", 1, 440, 44_100, 0.5},
		{"frequency just below Nyquist", 1, 22_049, 44_100, 0.5},
		{"frequency just below Nyquist with odd sampling rate", 1, 2_200, 4_401, 0.5},
		{"narrow pulse width 0.01", 1, 440, 44_100, 0.01},
		{"wide pulse width 0.99", 1, 440, 44_100, 0.99},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewPulse(tt.amplitude, tt.frequency, tt.samplingRate, tt.pulseWidth)
			assert.NoError(t, err)
			assert.NotNil(t, osc)
		})
	}
}

func TestPulse_Next(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    uint
		samplingRate int
		pulseWidth   float64
	}{
		{"pulse width 0.2", 1, 100, 4_410, 0.2},
		{"pulse width 0.25 at half amplitude", 0.5, 100, 4_410, 0.25},
		{"pulse width 0.5 is a square", 1, 100, 4_410, 0.5},
		{"pulse width 0.8", 1, 100, 4_410, 0.8},
		{"440 Hz at 44.1 kHz", 1, 440, 44_100, 0.25},
		{"just below Nyquist", 1, 22_049, 44_100, 0.25},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewPulse(tt.amplitude, tt.frequency, tt.samplingRate, tt.pulseWidth)
			require.NoError(t, err)

			// three full periods, so the phase wraps around 2PI at least twice
			samples := 3 * tt.samplingRate / int(tt.frequency)
			for i := range samples {
				// Sample i lies (i*f mod samplingRate) / samplingRate of the way into its period and is high while that is below pulseWidth.
				// The position is exact integer math. No case puts it closer than 2 to pulseWidth * samplingRate, so no sample sits on an edge.
				expected := tt.amplitude
				if float64(i*int(tt.frequency)%tt.samplingRate) >= tt.pulseWidth*float64(tt.samplingRate) {
					expected = -tt.amplitude
				}
				assert.Equal(t, expected, osc.next(), "sample %d", i)
			}
		})
	}
}

func TestPulse_NextSignedInt16(t *testing.T) {
	tests := []struct {
		name        string
		amplitude   float64
		pulseWidth  float64
		sampleIndex int
		expected    int16
	}{
		{"high at start", 1, 0.2, 0, 32_767},
		{"last high sample", 1, 0.2, 8, 32_767},
		{"first low sample", 1, 0.2, 9, -32_767},
		{"last low sample of the first period", 1, 0.2, 44, -32_767},
		{"high again after the phase wraps", 1, 0.2, 45, 32_767},
		{"wide pulse last high sample", 1, 0.8, 35, 32_767},
		{"wide pulse first low sample", 1, 0.8, 36, -32_767},
		{"half amplitude rounds half away from zero", 0.5, 0.2, 0, 16_384},
		{"negative half amplitude rounds half away from zero", 0.5, 0.2, 9, -16_384},
		{"quarter amplitude rounds up", 0.25, 0.2, 0, 8_192},
		{"negative quarter amplitude rounds down", 0.25, 0.2, 9, -8_192},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 100 Hz at 4410 Hz: sample i lies at 100*i mod 4410 of the 4410 positions in a period.
			// Pulse width 0.2 is high below position 882: samples 0-8 are high, 9-44 low, 45 (position 90) starts the next period.
			// Pulse width 0.8 is high below position 3528: sample 35 (3500) is high, 36 (3600) low.
			osc, err := NewPulse(tt.amplitude, 100, 4_410, tt.pulseWidth)
			require.NoError(t, err)

			for range tt.sampleIndex {
				osc.nextSignedInt16()
			}
			assert.Equal(t, tt.expected, osc.nextSignedInt16())
		})
	}
}

func TestPulse_Read(t *testing.T) {
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
			osc, err := NewPulse(1, 440, 44_100, 0.2)
			require.NoError(t, err)
			twin, err := NewPulse(1, 440, 44_100, 0.2)
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

func TestPulse_Read_encoding(t *testing.T) {
	tests := []struct {
		name          string
		amplitude     float64
		sampleIndex   int
		expectedBytes []byte
	}{
		{"positive peak 32767 = 0x7FFF", 1, 0, []byte{0xFF, 0x7F}},
		{"negative peak -32767 = 0x8001 in two's complement", 1, 9, []byte{0x01, 0x80}},
		{"quarter amplitude 8192 = 0x2000", 0.25, 0, []byte{0x00, 0x20}},
		{"negative quarter amplitude -8192 = 0xE000 in two's complement", 0.25, 9, []byte{0x00, 0xE0}},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 100 Hz at 4410 Hz with pulse width 0.2: samples 0-8 are high, 9-44 low
			osc, err := NewPulse(tt.amplitude, 100, 4_410, 0.2)
			require.NoError(t, err)

			p := make([]byte, 2*(tt.sampleIndex+1))
			_, err = osc.Read(p)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBytes, p[len(p)-2:])
		})
	}
}

func TestPulse_Read_continuity(t *testing.T) {
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
			whole, err := NewPulse(1, 440, 44_100, 0.2)
			require.NoError(t, err)
			chunked, err := NewPulse(1, 440, 44_100, 0.2)
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

func TestPulse_Read_shortBuffer(t *testing.T) {
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
			osc, err := NewPulse(1, 440, 44_100, 0.2)
			require.NoError(t, err)

			n, err := osc.Read(make([]byte, tt.bufferLength))
			assert.Equal(t, 0, n)
			assert.ErrorIs(t, err, io.ErrShortBuffer)
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

func BenchmarkPulse_NextSignedInt16(b *testing.B) {
	osc, err := NewPulse(1, 440, 44_100, 0.2)
	require.NoError(b, err)

	for b.Loop() {
		osc.nextSignedInt16()
	}
}

func BenchmarkPulse_Read(b *testing.B) {
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
			osc, err := NewPulse(1, 440, 44_100, 0.2)
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
