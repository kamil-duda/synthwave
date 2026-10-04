# AGENTS.md

This file provides guidance to AI coding agents working with code in this repository.

## Commands

- `make run`: play the synth (opens the audio device and plays until killed)
- `make run-gc-flags`: run with `-gcflags="-m -m"` to inspect escape analysis / inlining
- `make test`: `go test -v ./...`
- `make bench`: benchmarks only (`-bench . -benchmem -run ^$`)
- `make coverage`: writes `coverage.html` and opens it
- `make vet`: `go vet ./...` static analysis
- `gofmt -l .`: must print nothing (no Makefile target)
- `make mod` / `make update`: `go mod tidy` / upgrade dependencies

Single test or benchmark:

```
go test -v -run 'TestNewSineValidation/zero_frequency' ./oscillator
go test -v -bench BenchmarkName -benchmem -run '^$' ./oscillator
```

## Architecture

A Go audio synthesizer (module `synthwave`, Go 1.27) built on `github.com/ebitengine/oto/v3`.

Files:
- `main.go`: entry point. Creates the oscillator, the oto context and a player, then polls every 10 ms while the player is playing and panics on an oto context error. Audio settings are constants: `samplingRate` (44.1 kHz), `channels` (1, mono), `hardwareBufferSize` (50 ms OS buffer), `bufferSizeSamples` (oto player buffer; despite the name it is 4096 bytes, see below).
- `oscillator/oscillator.go`: the `Oscillator` interface and the `angularFrequency` helper.
- `oscillator/sine.go`: `NewSine` and the unexported `sine` struct.
- `*_test.go` next to each file: tests and benchmarks (`main` has none; it needs an audio device).

oto uses a pull model: `main.go` creates an oto context and passes an oscillator to `otoCtx.NewPlayer`. The player then calls the oscillator's `Read([]byte)` whenever the audio buffer needs refilling. Each oscillator is therefore an `io.Reader` that encodes generated samples directly into the byte stream.

Data flow during playback: oto's player calls `Read(p)` with 4096-byte buffers (`player.SetBufferSize(bufferSizeSamples)` takes bytes, i.e. 2048 samples, ~46 ms) -> `Read` calls `nextSignedInt16()` once per 2 bytes -> `nextSignedInt16()` scales `next()` to int16 -> `next()` returns `amplitude * sin(phase)` and advances the phase. A sine never returns `io.EOF`, so `make run` plays until killed.

**The sample format is an implicit contract between `main.go` and `Read`.** The oto context is set to 44.1 kHz, mono, `oto.FormatSignedInt16LE`. `sine.Read` writes one little-endian int16 (2 bytes) per sample and does not interleave channels. Changing the channel count or sample format in `main.go` means changing the encoding in `Read` too, or the audio will be garbled.

Oscillator pipeline (`oscillator/`):
- `NewSine` validates its arguments: amplitude in [0, 1] (NaN rejected), frequency > 0 and below the Nyquist frequency (`samplingRate/2`, compared as floats so odd sampling rates work), samplingRate > 0. It returns the interface rather than the concrete unexported struct.
- `next()` returns a float sample in `[-amplitude, amplitude]` and advances a phase accumulator by `phaseStep = 2PIf / samplingRate`, wrapping at 2PI with a single subtraction. That is enough only because the Nyquist check keeps `phaseStep < PI`.
- `nextSignedInt16()` scales that sample to int16 with `math.Round`, so values stay in [-32767, 32767].
- `Read` fills the buffer two bytes at a time, returns `io.ErrShortBuffer` for `len(p) < 2` and leaves a trailing odd byte untouched.

Errors: any non-EOF error from `Read` makes oto close the player, so `IsPlaying()` turns false and `main` exits without reporting it (it checks `otoCtx.Err()`, not `player.Err()`).

The `Oscillator` interface includes unexported methods (`next`, `nextSignedInt16`), so new oscillator types must live in the `oscillator` package.

Keep this section current: when a change adds or moves files, alters the data flow, the audio settings in `main.go`, the `Oscillator` interface or the validation rules, update this section in the same change.

## Conventions

- Tests sit in the same package (white-box) so they can call unexported methods. They are table-driven with `t.Parallel()` and use `testify/assert` (`require` for setup such as `NewSine` errors). Use `assert.InEpsilon`/`InDelta` for float comparisons.
- All code must be covered by tests. Prefer TDD: write a failing test first, then the implementation. Every change ships with a test that fails if the change is reverted.
- Interface methods (e.g. every `Oscillator` method: `Read`, `next`, `nextSignedInt16`) also need benchmarks. Use `for b.Loop()` (plus `b.SetBytes` for byte-oriented methods). All three are currently 0 allocs/op; treat a new allocation as a regression.
- Validation tests assert the specific message (`assert.ErrorContains`) and keep the other params valid (e.g. 440 Hz / 44_100), because checks mask each other (a negative `samplingRate` also trips the Nyquist check).
- Use plain ASCII in code, comments, docs and messages: `PI`/`2PI` not `π`, `->` not `→`, `-` not `—`. Don't comment self-explanatory code; do explain DSP concepts (e.g. Nyquist).
- Branches are named `<issue>-<slug>` (e.g. `3-sine-wave`) and commit messages start with the GitHub issue number (`#3 Refactor ...`).
- Only the user commits and pushes. Never run `git commit` or `git push`; leave changes in the working tree.
