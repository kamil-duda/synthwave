package oscillator

import (
	"github.com/stretchr/testify/assert"

	"math"
	"testing"
)

func TestAngularFrequency(t *testing.T) {
	tests := []struct {
		name             string
		frequency        uint
		angularFrequency float64
	}{
		{"1 Hz", 1, 1 * 2 * math.Pi},
		{"2 Hz", 2, 2 * 2 * math.Pi},
		{"100 Hz", 100, 100 * 2 * math.Pi},
		{"2 kHz", 2_000, 2_000 * 2 * math.Pi},
		{"20 kHz", 20_000, 20_000 * 2 * math.Pi},
		{"40 kHz", 40_000, 40_000 * 2 * math.Pi},
		{"200 kHz", 200_000, 200_000 * 2 * math.Pi},
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
