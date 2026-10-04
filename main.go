package main

import (
	"synthwave/oscillator"
	"time"

	"github.com/ebitengine/oto/v3"
)

const samplingRate = 44100                       // 44.1 kHz
const bufferSizeSamples = 4096                   // audio driver buffer size
const hardwareBufferSize = 50 * time.Millisecond // length of the operating system buffer
const channels = 1                               // 1 - mono, 2 - stereo

func main() {
	sineOscillator, err := oscillator.NewSine(0.2, 440, samplingRate)
	if err != nil {
		panic("Error creating sine oscillator: " + err.Error())
	}

	ctxOptions := &oto.NewContextOptions{}
	ctxOptions.SampleRate = samplingRate
	ctxOptions.ChannelCount = channels
	ctxOptions.Format = oto.FormatSignedInt16LE
	ctxOptions.BufferSize = hardwareBufferSize

	otoCtx, readyChan, err := oto.NewContext(ctxOptions)
	if err != nil {
		panic("Creating oto context failed: " + err.Error())
	}
	// Wait for the hardware to be ready
	<-readyChan

	player := otoCtx.NewPlayer(sineOscillator)
	player.SetBufferSize(bufferSizeSamples)
	player.Play()

	for player.IsPlaying() {
		if err := otoCtx.Err(); err != nil {
			panic("oto error: " + err.Error())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
