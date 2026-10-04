# Issues

Work tracker based on GitHub issues, snapshot from 2026-10-04.
`[x]` means done here, even if the issue is still open on GitHub. Nested items are GitHub sub-issues.

- [x] **#1** Experiment with `oto` to understand audio data format - play some basic tone
- [ ] **#2** Implement oscillators for standard waveforms
  - [x] **#3** Sine wave
  - [x] **#4** Square wave
  - [ ] **#4b** Advanced square wave
  - [ ] **#5** Sawtooth wave
  - [ ] **#6** Triangle wave
  - [ ] Extract code shared by oscillators (validation, phase accumulator, int16 encoding in `Read`) and their common tests
- [ ] **#7** Implement a basic signal generator interface for output to `oto`
- [ ] **#8** Implement a single wave player with frequency and volume control
- [ ] **#9** Add simple TUI input to change frequency and volume
- [ ] **#10** Implement frequency calculation based on A440
- [ ] **#11** Add ability to modify the reference frequency
- [ ] **#12** Map PC keyboard keys to a piano octave
- [ ] **#13** Implement an Envelope Generator - ADSR
  - [ ] **#14** Attack - Time for the sound to reach peak volume
  - [ ] **#15** Decay - Time for the sound to drop to sustain level
  - [ ] **#16** Sustain - Constant volume level while the key is held
  - [ ] **#17** Release - Time for the sound to fade out after the key is released
- [ ] **#18** Add ability to generate multiple signals in oscilator
- [ ] **#19** Implement an audio mixer to combine multiple signals
- [ ] **#20** Add TUI elements for controlling the mixer
- [ ] **#21** TUI - Display piano keys
- [ ] **#22** TUI - experiment with mouse support
- [ ] **#23** Add effects - Amplitude modulation
- [ ] **#24** Add effects - Frequency modulation
- [ ] **#25** Effects - Vibrato
- [ ] **#26** Effects - Distortion
- [ ] **#27** Effects - Echo
- [ ] **#28** Effects - Reverb
- [ ] **#29** Effects - Tremolo
- [ ] **#30** Effects - Filters: low-pass, high-pass, band-pass
- [ ] **#31** Add TUI visualization for waveforms
- [ ] **#32** Add ability to save/load presets
- [ ] **#33** Implement native macOS audio interaction (CoreAudio) to replace or complement `oto`
- [ ] **#37** Optimize oscillator functions
- [ ] **#38** Octave switcher
