package main

import (
	"math"
	"time"

	"github.com/ebitengine/oto/v3"
)

const sampleRate = 44100
const bufferSizeSamples = 4096
const hardwareBufferSize = 50 * time.Millisecond // length of the operating system buffer
const channels = 1

// SinOscillator generates sinusoidal waveforms for audio synthesis.
// It maintains phase information to produce continuous sine waves at a specified frequency.
type SinOscillator struct {
	// amplitude is the amplitude of the oscillator's waveform, between 0 and 1
	amplitude float64
	// frequency is the oscillator's frequency in Hz
	frequency uint16
	// angularFrequency is the oscillator's angular frequency in radians per second
	angularFrequency float64
	// phase is the current, internal phase angle in radians
	phase float64
	// phaseStep is the phase increment per oen sample, calculated as angular frequency / sample rate
	phaseStep float64
}

func newSinOscillator(amplitude float64, frequency uint16) *SinOscillator {
	if amplitude < 0 || 1 <= amplitude {
		panic("amplitude must be between 0 and 1")
	}

	angFreq := angularFrequency(frequency)
	return &SinOscillator{
		amplitude:        amplitude,
		frequency:        frequency,
		angularFrequency: angFreq,
		phase:            0,
		phaseStep:        angFreq / float64(sampleRate),
	}
}

// todo: test
// angularFrequency converts frequency f [periods/second] into angular frequency [rad/second].
// One full period (360 deg) is 2 PI rad.
// Having 1Hz means one full period per second, so an angular frequency of 2 PI rad / second.
// Higher frequency means more angular frequency to keep up.
func angularFrequency(f uint16) float64 {
	return 2 * math.Pi * float64(f)
}

// todo: test
// todo: benchmark
func (s *SinOscillator) next() float64 {
	s.phase += s.phaseStep
	if s.phase >= 2*math.Pi {
		s.phase -= 2 * math.Pi
	}
	return s.amplitude * math.Sin(s.phase)
}

func (s *SinOscillator) nextSignedInt16() int16 {
	return int16(math.Round(s.next() * math.MaxInt16))
}

func (s *SinOscillator) Read(p []byte) (n int, err error) {
	pLength := len(p)
	pIdx := 0

	// while index of next element is smaller than the length e.g. (pIdx=4, pIdx+1=5, pLength=5)
	for pIdx+1 < pLength {
		sample := s.nextSignedInt16()
		p[pIdx] = byte(sample)
		p[pIdx+1] = byte(sample >> 8)
		pIdx += 2
	}
	return pIdx, err
}

func main() {
	oscillator := newSinOscillator(0.2, 440)

	ctxOptions := &oto.NewContextOptions{}
	ctxOptions.SampleRate = sampleRate
	ctxOptions.ChannelCount = channels
	ctxOptions.Format = oto.FormatSignedInt16LE
	ctxOptions.BufferSize = hardwareBufferSize

	otoCtx, readyChan, err := oto.NewContext(ctxOptions)
	if err != nil {
		panic("Creating oto context failed: " + err.Error())
	}
	// Wait for the hardware to be ready
	<-readyChan

	//
	player := otoCtx.NewPlayer(oscillator)
	player.SetBufferSize(bufferSizeSamples)
	player.Play()

	// We can wait for the sound to finish playing using something like this
	for player.IsPlaying() {
		if err := otoCtx.Err(); err != nil {
			panic("oto error: " + err.Error())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
