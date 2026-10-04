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

func TestNewSineValidation(t *testing.T) {
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
			_, err := NewSine(tt.amplitude, tt.frequency, tt.samplingRate)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func TestNewSineBoundaries(t *testing.T) {
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
			osc, err := NewSine(tt.amplitude, tt.frequency, tt.samplingRate)
			assert.NoError(t, err)
			assert.NotNil(t, osc)
		})
	}
}

func TestNext(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    uint
		samplingRate int
	}{
		{"full amplitude 100 Hz", 1, 100, 4_400},
		{"half amplitude 100 Hz", 0.5, 100, 4_400},
		{"zero amplitude", 0, 100, 4_400},
		{"440 Hz at 44.1 kHz", 1, 440, 44_100},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSine(tt.amplitude, tt.frequency, tt.samplingRate)
			require.NoError(t, err)

			// three full periods, so the phase wraps around 2PI at least twice
			samples := 3 * tt.samplingRate / int(tt.frequency)
			for i := range samples {
				expected := tt.amplitude * math.Sin(2*math.Pi*float64(tt.frequency)*float64(i)/float64(tt.samplingRate))
				assert.InDelta(t, expected, osc.next(), 1e-9, "sample %d", i)
			}
		})
	}
}

func TestNextPhaseWrapping(t *testing.T) {
	tests := []struct {
		name         string
		frequency    uint
		samplingRate int
	}{
		{"440 Hz at 44.1 kHz", 440, 44_100},
		{"just below Nyquist", 22_049, 44_100},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSine(1, tt.frequency, tt.samplingRate)
			require.NoError(t, err)
			s := osc.(*sine)

			for i := range tt.samplingRate {
				s.next()
				require.GreaterOrEqual(t, s.phase, 0.0, "sample %d", i)
				require.Less(t, s.phase, 2*math.Pi, "sample %d", i)
			}
		})
	}
}

func TestNextSignedInt16(t *testing.T) {
	tests := []struct {
		name        string
		amplitude   float64
		sampleIndex int
		expected    int16
	}{
		{"zero at start", 1, 0, 0},
		{"rounds down", 1, 1, 4_663},
		{"rounds up", 1, 3, 13_612},
		{"positive peak", 1, 11, 32_767},
		{"zero at half period", 1, 22, 0},
		{"negative rounds away from zero", 1, 25, -13_612},
		{"negative peak", 1, 33, -32_767},
		{"quarter amplitude positive peak", 0.25, 11, 8_192},
		{"quarter amplitude negative peak", 0.25, 33, -8_192},
		{"zero amplitude", 0, 11, 0},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSine(tt.amplitude, 100, 4_400)
			require.NoError(t, err)

			for range tt.sampleIndex {
				osc.nextSignedInt16()
			}
			assert.Equal(t, tt.expected, osc.nextSignedInt16())
		})
	}
}

func TestSine_Read(t *testing.T) {
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
			osc, err := NewSine(1, 440, 44_100)
			require.NoError(t, err)
			twin, err := NewSine(1, 440, 44_100)
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

func TestSine_ReadEncoding(t *testing.T) {
	tests := []struct {
		name          string
		sampleIndex   int
		expectedBytes []byte
	}{
		{"zero", 0, []byte{0x00, 0x00}},
		{"positive 13612 = 0x352C", 3, []byte{0x2C, 0x35}},
		{"positive peak 32767 = 0x7FFF", 11, []byte{0xFF, 0x7F}},
		{"negative -13612 = 0xCAD4 in two's complement", 25, []byte{0xD4, 0xCA}},
		{"negative peak -32767 = 0x8001 in two's complement", 33, []byte{0x01, 0x80}},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			osc, err := NewSine(1, 100, 4_400)
			require.NoError(t, err)

			p := make([]byte, 2*(tt.sampleIndex+1))
			_, err = osc.Read(p)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBytes, p[len(p)-2:])
		})
	}
}

func TestSine_ReadContinuity(t *testing.T) {
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
			whole, err := NewSine(1, 440, 44_100)
			require.NoError(t, err)
			chunked, err := NewSine(1, 440, 44_100)
			require.NoError(t, err)

			expected := make([]byte, 1_200)
			_, err = whole.Read(expected)
			require.NoError(t, err)

			actual := make([]byte, 0, len(expected))
			chunk := make([]byte, tt.chunkLength)
			for len(actual) < len(expected) {
				n, err := chunked.Read(chunk)
				require.NoError(t, err)
				actual = append(actual, chunk[:n]...)
			}
			assert.Equal(t, expected, actual)
		})
	}
}

func TestSine_ReadShortBuffer(t *testing.T) {
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
			osc, err := NewSine(1, 440, 44_100)
			require.NoError(t, err)

			n, err := osc.Read(make([]byte, tt.bufferLength))
			assert.Equal(t, 0, n)
			assert.ErrorIs(t, err, io.ErrShortBuffer)
		})
	}
}

func BenchmarkNext(b *testing.B) {
	osc, err := NewSine(1, 440, 44_100)
	require.NoError(b, err)

	for b.Loop() {
		osc.next()
	}
}

func BenchmarkNextSignedInt16(b *testing.B) {
	osc, err := NewSine(1, 440, 44_100)
	require.NoError(b, err)

	for b.Loop() {
		osc.nextSignedInt16()
	}
}

func BenchmarkRead(b *testing.B) {
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
			osc, err := NewSine(1, 440, 44_100)
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
