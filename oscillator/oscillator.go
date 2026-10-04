package oscillator

import (
	"io"
	"math"
)

type Oscillator interface {
	io.Reader
	next() float64
	nextSignedInt16() int16
}

// angularFrequency converts frequency f [periods/second] into angular frequency [rad/second].
// One full period (360 deg) is 2 PI rad.
// Having 1Hz means one full period per second, so an angular frequency of 2 PI rad / second.
// Higher frequency means more angular frequency to keep up.
func angularFrequency(f float64) float64 {
	return 2 * math.Pi * f
}
