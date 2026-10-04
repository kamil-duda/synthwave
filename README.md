# synthwave


A small software synthesizer in Go, built on [oto](https://github.com/ebitengine/oto).

`make run` plays every oscillator in turn (2 s each), `make test` runs the tests and `make help` lists the other targets.
Planned work is in [Tasks](#tasks) below.

<!-- TOC -->
* [synthwave](#synthwave)
  * [Waveforms](#waveforms)
    * [Sine - `NewSine(amplitude, frequency, samplingRate)`](#sine---newsineamplitude-frequency-samplingrate)
    * [Square - `NewSquare(amplitude, frequency, samplingRate)`](#square---newsquareamplitude-frequency-samplingrate)
    * [Pulse - `NewPulse(amplitude, frequency, samplingRate, pulseWidth)`](#pulse---newpulseamplitude-frequency-samplingrate-pulsewidth)
    * [Sawtooth - `NewSawtooth(amplitude, frequency, samplingRate)`](#sawtooth---newsawtoothamplitude-frequency-samplingrate)
    * [Triangle - `NewTriangle(amplitude, frequency, samplingRate, symmetry)`](#triangle---newtriangleamplitude-frequency-samplingrate-symmetry)
  * [Tasks](#tasks)
<!-- TOC -->

## Waveforms

Every diagram shows two periods of an oscillator's output with amplitude A. Time runs to the right
and the left edge is phase 0, where the oscillator starts.

### Sine - `NewSine(amplitude, frequency, samplingRate)`

```
 +A |    .---.                   .---.
    |  .'     '.               .'     '.
    | /         \             /         \
  0 |/           \           /           \           /
    |             \         /             \         /
    |              '.     .'               '.     .'
 -A |                '---'                   '---'
     <------- 1 period ------>
```

Starts at 0 and rises. A single pure tone without harmonics.

### Square - `NewSquare(amplitude, frequency, samplingRate)`

```
 +A |------------+           +-----------+           +
    |            |           |           |           |
    |            |           |           |           |
  0 |            |           |           |           |
    |            |           |           |           |
    |            |           |           |           |
 -A |            +-----------+           +-----------+
     <------- 1 period ------>
```

+A for the first half of the period, -A for the second. Odd harmonics (f, 3f, 5f, ...) falling as 1/n.

### Pulse - `NewPulse(amplitude, frequency, samplingRate, pulseWidth)`

`pulseWidth` 0.2, as in `main.go`:

```
 +A |------+                       +-----+                       +
    |      |                       |     |                       |
    |      |                       |     |                       |
  0 |      |                       |     |                       |
    |      |                       |     |                       |
    |      |                       |     |                       |
 -A |      +-----------------------+     +-----------------------+
     <---------- 1 period --------->
```

+A for the first `pulseWidth` of the period, -A for the rest; 0.5 gives the square. Any other `pulseWidth`
moves the average away from 0 (a DC offset), e.g. to -0.6A at 0.2.

### Sawtooth - `NewSawtooth(amplitude, frequency, samplingRate)`

```
 +A |                      _.|                     _.|
    |                  _.-'  |                 _.-'  |
    |              _.-'      |             _.-'      |
  0 |          _.-'          |         _.-'          |
    |      _.-'              |     _.-'              |
    |  _.-'                  | _.-'                  |
 -A |-'                      -'                      -
     <------- 1 period ------>
```

Rises from -A towards +A, then jumps back. Every harmonic (f, 2f, 3f, ...) falling as 1/n, so it sounds bright and buzzy.

### Triangle - `NewTriangle(amplitude, frequency, samplingRate, symmetry)`

`symmetry` is the part of the period the wave rises. 0.5 is the classic triangle: odd harmonics falling as 1/n^2,
so it sounds close to a sine.

```
 +A |           ..                      ..
    |         .'  '.                  .'  '.
    |       .'      '.              .'      '.
  0 |     .'          '.          .'          '.
    |   .'              '.      .'              '.
    | .'                  '.  .'                  '.
 -A |'                      ''                      ''
     <------- 1 period ------>
```

0.2, as in `main.go`: a fast rise and a slow fall, brighter.

```
 +A |      ._                            ._
    |     /  '-._                       /  '-._
    |    /       '-._                  /       '-._
  0 |   /            '-._             /            '-._
    |  /                 '-._        /                 '-._
    | /                      '-._   /                      '-._
 -A |/                           '-/                           '-/
     <---------- 1 period --------->
```

`symmetry` 1 is the sawtooth above and 0 a falling sawtooth.

## Tasks

- [x] **#1** Experiment with `oto` to understand audio data format - play some basic tone
- [ ] **#2** Implement oscillators for standard waveforms
  - [x] **#3** Sine wave
  - [x] **#4** Square wave
  - [x] **#4b** Pulse wave
  - [x] **#5** Sawtooth wave
  - [x] **#6** Triangle wave
  - [x] **#39** Change `frequency` from `uint` to `float64` (notes based on A440 are not whole numbers, e.g. C4 = 261.63 Hz)
  - [x] **#40** Extract code shared by oscillators (validation, phase accumulator, int16 encoding in `Read`) and their common tests
- [ ] **#7** Implement a basic signal generator interface for output to `oto`
- [ ] **#41** White noise source: random samples in [-amplitude, amplitude] with no frequency or phase (percussion, hi-hats, wind), seedable so tests are deterministic
- [ ] **#42** DC-blocking filter right after each oscillator, before the envelope and mixing: one-pole high-pass `y[n] = x[n] - x[n-1] + R*y[n-1]` with R ~ 0.999 (cutoff ~7 Hz; 0.995 would be ~35 Hz and thin the bass) that removes the DC offset (the signal's average, e.g. -0.6 * amplitude of a 0.2 pulse wave)
- [ ] **#10** Implement frequency calculation based on A440
- [ ] **#8** Implement a single wave player with frequency and volume control
  - [ ] **#43** Change parameters safely while playing: no data race between the UI and oto's `Read`, smoothed volume changes (no zipper noise), phase-continuous frequency changes (no clicks)
- [ ] **#44** Spike: can the terminal report key release (e.g. kitty keyboard protocol)? Decides how #12 and #13 get note on / note off
- [ ] **#45** TUI basics: library choice, main loop, quit key, terminal restored on exit
- [ ] **#9** Add simple TUI input to change frequency and volume
- [ ] **#46** Lower the latency for live play (now ~100 ms: player buffer ~46 ms + OS buffer 50 ms)
- [ ] **#12** Map PC keyboard keys to a piano octave
- [ ] **#38** Octave switcher
- [ ] **#21** TUI - Display piano keys
- [ ] **#11** Add ability to modify the reference frequency
- [ ] **#13** Implement an Envelope Generator - ADSR
  - [ ] **#14** Attack - Time for the sound to reach peak volume
  - [ ] **#15** Decay - Time for the sound to drop to sustain level
  - [ ] **#16** Sustain - Constant volume level while the key is held
  - [ ] **#17** Release - Time for the sound to fade out after the key is released
- [ ] **#30** Effects - Filters: low-pass, high-pass, band-pass
- [ ] **#18** Add ability to generate multiple signals in oscilator
- [ ] **#19** Implement an audio mixer to combine multiple signals
  - [ ] **#47** Master gain and clipping protection (a sum of signals can exceed [-1, 1])
- [ ] **#20** Add TUI elements for controlling the mixer
- [ ] **#48** LFO - low-frequency oscillator as a modulation source (for #29, #25 and #49)
- [ ] **#29** Effects - Tremolo
- [ ] **#25** Effects - Vibrato
- [ ] **#49** PWM - modulate the pulse wave's `pulseWidth` with the LFO (#48)
- [ ] **#23** Add effects - Amplitude modulation
- [ ] **#24** Add effects - Frequency modulation
- [ ] **#26** Effects - Distortion
- [ ] **#27** Effects - Echo
- [ ] **#28** Effects - Reverb
- [ ] **#31** Add TUI visualization for waveforms
- [ ] **#50** Band-limited square, pulse and sawtooth (PolyBLEP) to remove aliasing
- [ ] **#37** Optimize oscillator functions
- [ ] **#32** Add ability to save/load presets
- [ ] **#22** TUI - experiment with mouse support
- [ ] **#33** Implement native macOS audio interaction (CoreAudio) to replace or complement `oto`
