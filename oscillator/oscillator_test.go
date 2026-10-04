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

// constructors lists every oscillator with its extra parameter fixed, so the shared tests cover all of them.
// A new oscillator belongs here.
var constructors = []struct {
	name   string
	create func(amplitude, frequency float64, samplingRate int) (Oscillator, error)
}{
	{"sine", NewSine},
	{"square", NewSquare},
	{"pulse", func(amplitude, frequency float64, samplingRate int) (Oscillator, error) {
		return NewPulse(amplitude, frequency, samplingRate, 0.2)
	}},
	{"sawtooth", NewSawtooth},
	{"triangle", func(amplitude, frequency float64, samplingRate int) (Oscillator, error) {
		return NewTriangle(amplitude, frequency, samplingRate, 0.2)
	}},
}

func TestOscillator_validation(t *testing.T) {
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
	for _, c := range constructors {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					_, err := c.create(tt.amplitude, tt.frequency, tt.samplingRate)
					assert.ErrorContains(t, err, tt.expectedError)
				})
			}
		})
	}
}

func TestOscillator_boundaries(t *testing.T) {
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
	for _, c := range constructors {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					osc, err := c.create(tt.amplitude, tt.frequency, tt.samplingRate)
					assert.NoError(t, err)
					assert.NotNil(t, osc)
				})
			}
		})
	}
}

func TestAngularFrequency(t *testing.T) {
	tests := []struct {
		name             string
		frequency        float64
		angularFrequency float64
	}{
		{"1 Hz", 1, 1 * 2 * math.Pi},
		{"2 Hz", 2, 2 * 2 * math.Pi},
		{"100 Hz", 100, 100 * 2 * math.Pi},
		{"2 kHz", 2_000, 2_000 * 2 * math.Pi},
		{"20 kHz", 20_000, 20_000 * 2 * math.Pi},
		{"40 kHz", 40_000, 40_000 * 2 * math.Pi},
		{"200 kHz", 200_000, 200_000 * 2 * math.Pi},
		{"fractional 261.63 Hz (C4)", 261.63, 261.63 * 2 * math.Pi},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.angularFrequency
			actual := angularFrequency(tt.frequency)
			assert.InEpsilon(t, expected, actual, 1e-15)
		})
	}
}

func TestPhaseTracker_Advance(t *testing.T) {
	tests := []struct {
		name         string
		frequency    float64
		samplingRate int
	}{
		{"440 Hz at 44.1 kHz", 440, 44_100},
		{"fractional 261.63 Hz (C4)", 261.63, 44_100},
		{"fractional frequency below 1 Hz", 0.5, 44_100},
		{"just below Nyquist", 22_049, 44_100},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := newPhaseTracker(tt.frequency, tt.samplingRate)

			// one second of samples
			for i := range tt.samplingRate {
				// Sample i lies i*f/samplingRate periods after the start: at phase 2PI*i*f/samplingRate, minus whole periods.
				// advance() returns the phase before it moves on, so the first sample is at phase 0.
				expected := 2 * math.Pi * float64(i) * tt.frequency / float64(tt.samplingRate)
				phase := a.advance()
				require.GreaterOrEqual(t, phase, 0.0, "sample %d", i)
				require.Less(t, phase, 2*math.Pi, "sample %d", i)
				// math.Remainder compares the angles on the circle, so 2PI - 1e-12 and 0 count as equal
				require.InDelta(t, 0, math.Remainder(phase-expected, 2*math.Pi), 1e-9, "sample %d", i)
			}
		})
	}
}

func TestToSignedInt16(t *testing.T) {
	tests := []struct {
		name     string
		sample   float64
		expected int16
	}{
		{"zero", 0, 0},
		{"positive peak", 1, 32_767},
		{"negative peak", -1, -32_767},
		{"0.5 is 16383.5, rounded half away from zero", 0.5, 16_384},
		{"-0.5 is -16383.5, rounded half away from zero", -0.5, -16_384},
		{"0.1 is 3276.7, rounded up", 0.1, 3_277},
		{"0.2 is 6553.4, rounded down", 0.2, 6_553},
		{"-0.2 is -6553.4, rounded toward zero", -0.2, -6_553},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, toSignedInt16(tt.sample))
		})
	}
}

func TestEncode(t *testing.T) {
	tests := []struct {
		name          string
		bufferLength  int
		expectedN     int
		expectedBytes []byte
	}{
		{"one sample", 2, 2, []byte{0xFF, 0x7F}},
		{"four samples", 8, 8, []byte{0xFF, 0x7F, 0x01, 0x80, 0x00, 0x20, 0x00, 0xE0}},
		{"odd length leaves last byte untouched", 7, 6, []byte{0xFF, 0x7F, 0x01, 0x80, 0x00, 0x20, 0xAA}},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 1 -> 32767 = 0x7FFF, -1 -> -32767 = 0x8001 in two's complement,
			// 0.25 -> 8192 = 0x2000, -0.25 -> -8192 = 0xE000 in two's complement. Little-endian writes the low byte first.
			samples := []float64{1, -1, 0.25, -0.25}
			calls := 0
			next := func() float64 {
				sample := samples[calls]
				calls++
				return sample
			}

			p := bytes.Repeat([]byte{0xAA}, tt.bufferLength)
			n, err := encode(p, next)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedN, n)
			assert.Equal(t, tt.expectedBytes, p)
			assert.Equal(t, tt.expectedN/2, calls, "next must be called once per written sample")
		})
	}
}

func TestEncode_shortBuffer(t *testing.T) {
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
			calls := 0
			n, err := encode(make([]byte, tt.bufferLength), func() float64 { calls++; return 0 })
			assert.Equal(t, 0, n)
			assert.ErrorIs(t, err, io.ErrShortBuffer)
			assert.Zero(t, calls, "next must not be called without room for a sample")
		})
	}
}

func TestOscillator_Read(t *testing.T) {
	tests := []struct {
		name         string
		bufferLength int
	}{
		{"one sample", 2},
		{"half a second at 44.1 kHz", 44_100},
	}

	t.Parallel()
	for _, c := range constructors {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					osc, err := c.create(1, 440, 44_100)
					require.NoError(t, err)
					twin, err := c.create(1, 440, 44_100)
					require.NoError(t, err)

					p := make([]byte, tt.bufferLength)
					n, err := osc.Read(p)
					require.NoError(t, err)
					require.Equal(t, tt.bufferLength, n)

					// Read must deliver this oscillator's own samples, in order
					expected := make([]int16, n/2)
					actual := make([]int16, n/2)
					for i := range expected {
						expected[i] = toSignedInt16(twin.next())
						actual[i] = int16(binary.LittleEndian.Uint16(p[2*i:]))
					}
					assert.Equal(t, expected, actual)
				})
			}
		})
	}
}

func TestOscillator_Read_continuity(t *testing.T) {
	tests := []struct {
		name        string
		chunkLength int
	}{
		{"one sample per read", 2},
		{"three samples per read", 6},
		{"five bytes per read (odd)", 5},
		{"two hundred samples per read", 400},
	}

	t.Parallel()
	for _, c := range constructors {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					whole, err := c.create(1, 440, 44_100)
					require.NoError(t, err)
					chunked, err := c.create(1, 440, 44_100)
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
					assert.Equal(t, expected, actual[:len(expected)])
				})
			}
		})
	}
}
