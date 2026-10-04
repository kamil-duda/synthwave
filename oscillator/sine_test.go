package oscillator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSineValidation(t *testing.T) {
	tests := []struct {
		name         string
		amplitude    float64
		frequency    uint
		samplingRate int
	}{
		{"negative amplitude -0.1", -0.1, 1, 1},
		{"negative amplitude -0.01", -0.01, 1, 1},
		{"negative amplitude -1", -1, 1, 1},
		{"negative amplitude -10", -10, 1, 1},
		{"zero frequency", 1, 0, 1},
		{"zero sampling rate", 1, 1, 0},
		{"negative sampling rate -1", 1, 1, -1},
		{"negative sampling rate -10", 1, 1, -10},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewSine(tt.amplitude, tt.frequency, tt.samplingRate)
			assert.Error(t, err)
		})
	}
}

func TestNext(t *testing.T) {
	oscillator, _ := NewSine(1, 100, 4400)
	for i := 0; i < 20; i++ {
		assert.Equal(t, 1, oscillator.next())
	}

}

func TestNextSignedInt16(t *testing.T) {}

func TestSine_Read(t *testing.T) {
}
