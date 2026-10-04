package main

import (
	"log"
	"synthwave/oscillator"
	"time"

	"github.com/ebitengine/oto/v3"
)

const samplingRate = 44100                       // 44.1 kHz
const bufferSizeSamples = 4096                   // audio driver buffer size
const hardwareBufferSize = 50 * time.Millisecond // length of the operating system buffer
const channels = 1                               // 1 - mono, 2 - stereo
const volume = 0.2                               // shared by all oscillators, so only the waveform changes
const baseFrequency = 440                        // shared by all oscillators, so only the waveform changes
const pulseWidth = 0.2                           // part of the period the pulse wave is high (0.5 would be a square)
const triangleSymmetry = 0.2                     // part of the period the triangle wave rises (0.5 would be the classic triangle, close to a sine; 1 a sawtooth)
const switchInterval = 2 * time.Second           // how long each oscillator plays before switching to the next one

func main() {
	sineOscillator, err := oscillator.NewSine(volume, baseFrequency, samplingRate)
	if err != nil {
		panic("Error creating sine oscillator: " + err.Error())
	}
	squareOscillator, err := oscillator.NewSquare(volume, baseFrequency, samplingRate)
	if err != nil {
		panic("Error creating square oscillator: " + err.Error())
	}
	pulseOscillator, err := oscillator.NewPulse(volume, baseFrequency, samplingRate, pulseWidth)
	if err != nil {
		panic("Error creating pulse oscillator: " + err.Error())
	}
	sawtoothOscillator, err := oscillator.NewSawtooth(volume, baseFrequency, samplingRate)
	if err != nil {
		panic("Error creating sawtooth oscillator: " + err.Error())
	}
	triangleOscillator, err := oscillator.NewTriangle(volume, baseFrequency, samplingRate, triangleSymmetry)
	if err != nil {
		panic("Error creating triangle oscillator: " + err.Error())
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

	oscillators := []oscillator.Oscillator{sineOscillator, squareOscillator, pulseOscillator, sawtoothOscillator, triangleOscillator}
	players := make([]*oto.Player, len(oscillators))
	for i, osc := range oscillators {
		players[i] = otoCtx.NewPlayer(osc)
		players[i].SetBufferSize(bufferSizeSamples)
	}
	active := 0
	players[active].Play()
	log.Printf("playing %v", oscillators[active])

	switchTicker := time.NewTicker(switchInterval)
	errorTicker := time.NewTicker(10 * time.Millisecond)
	for players[active].IsPlaying() {
		select {
		case <-switchTicker.C:
			// A paused player keeps its buffered samples, so the oscillator resumes where it stopped.
			// The switch cuts the wave at an arbitrary point of its period, which can be heard as a click.
			players[active].Pause()
			active = (active + 1) % len(players)
			players[active].Play()
			log.Printf("playing %v", oscillators[active])
		case <-errorTicker.C:
			if err := otoCtx.Err(); err != nil {
				panic("oto error: " + err.Error())
			}
		}
	}
}
