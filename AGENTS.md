# AGENTS.md

This file provides guidance to AI coding agents working with code in this repository.

## Commands

- `make run`: play the synth (opens the audio device and alternates sine and square every 5 s until killed)
- `make run-gc-flags`: run with `-gcflags="-m -m"` to inspect escape analysis / inlining
- `make test`: `go test -v ./...`
- `make bench`: benchmarks only (`-bench . -benchmem -run ^$`)
- `make coverage`: writes `coverage.html` and opens it
- `make vet`: `go vet ./...` static analysis
- `gofmt -l .`: must print nothing (no Makefile target)
- `make mod` / `make update`: `go mod tidy` / upgrade dependencies

Single test or benchmark:

```
go test -v -run 'TestNewSine_validation/zero_frequency' ./oscillator
go test -v -bench BenchmarkName -benchmem -run '^$' ./oscillator
```

## Architecture

A Go audio synthesizer (module `synthwave`, Go 1.27) built on `github.com/ebitengine/oto/v3`.

Files:
- `main.go`: entry point. Creates a sine and a square oscillator with the same `volume` (0.2, passed as their amplitude) and `baseFrequency` (440 Hz), the oto context and one player per oscillator. It plays the sine first and every `switchInterval` (5 s) pauses the active player and plays the next one. A paused player keeps its buffered samples, so each oscillator resumes where it stopped. The switch is not sample-accurate and can click. Meanwhile it polls every 10 ms, panics on an oto context error and exits when the active player stops playing. Audio settings are constants: `samplingRate` (44.1 kHz), `channels` (1, mono), `hardwareBufferSize` (50 ms OS buffer), `bufferSizeSamples` (oto player buffer; despite the name it is 4096 bytes, see below).
- `oscillator/oscillator.go`: the `Oscillator` interface and the `angularFrequency` helper.
- `oscillator/sine.go`: `NewSine` and the unexported `sine` struct.
- `oscillator/basic_square.go`: `NewSquare` and the unexported `square` struct, a basic square wave: symmetric (50% duty cycle) and naive (not band-limited).
- `*_test.go` next to each file: tests and benchmarks (`main` has none; it needs an audio device).

oto uses a pull model: `main.go` creates an oto context and passes an oscillator to `otoCtx.NewPlayer`. The player then calls the oscillator's `Read([]byte)` whenever the audio buffer needs refilling. Each oscillator is therefore an `io.Reader` that encodes generated samples directly into the byte stream.

Data flow during playback: oto's player calls `Read(p)` with 4096-byte buffers (`player.SetBufferSize(bufferSizeSamples)` takes bytes, i.e. 2048 samples, ~46 ms) -> `Read` calls `nextSignedInt16()` once per 2 bytes -> `nextSignedInt16()` scales `next()` to int16 -> `next()` returns the waveform's value at the current phase and advances the phase. Oscillators never return `io.EOF`, so `make run` plays until killed.

**The sample format is an implicit contract between `main.go` and `Read`.** The oto context is set to 44.1 kHz, mono, `oto.FormatSignedInt16LE`. Each oscillator's `Read` (`sine.Read`, `square.Read`) writes one little-endian int16 (2 bytes) per sample and does not interleave channels. Changing the channel count or sample format in `main.go` means changing the encoding in `Read` too, or the audio will be garbled.

Oscillator pipeline (`oscillator/`). `sine` and `square` currently duplicate the validation, the phase accumulator and `Read` (extracting them is a task in `ISSUES.md`), so a fix in one usually belongs in the other too:
- `NewSine` and `NewSquare` validate their arguments the same way: amplitude in [0, 1] (NaN rejected), frequency > 0 and below the Nyquist frequency (`samplingRate/2`, compared as floats so odd sampling rates work), samplingRate > 0. They return the interface rather than the concrete unexported struct.
- `next()` returns a float sample in `[-amplitude, amplitude]` and advances a phase accumulator by `phaseStep = 2PIf / samplingRate`, wrapping at 2PI with a single subtraction. That is enough only because the Nyquist check keeps `phaseStep < PI`. `sine` returns `amplitude * sin(phase)`. `square` returns `+amplitude` for phase in [0, PI) and `-amplitude` in [PI, 2PI). Its harmonics above the Nyquist frequency alias, and the check cannot prevent that (see the `square` doc comment).
- `nextSignedInt16()` scales that sample to int16 with `math.Round`, so values stay in [-32767, 32767].
- `Read` fills the buffer two bytes at a time, returns `io.ErrShortBuffer` for `len(p) < 2` and leaves a trailing odd byte untouched.

Errors: any non-EOF error from `Read` makes oto close the player, so `IsPlaying()` turns false and `main` exits without reporting it (it checks `otoCtx.Err()`, not `player.Err()`). An error in a paused player's `Read` shows up at the switch, because `Play()` does nothing on a player with an error.

The `Oscillator` interface includes unexported methods (`next`, `nextSignedInt16`), so new oscillator types must live in the `oscillator` package.

Keep this section current: when a change adds or moves files, alters the data flow, the audio settings in `main.go`, the `Oscillator` interface or the validation rules, update this section in the same change.

## Conventions

- Tests sit in the same package (white-box) so they can call unexported methods. They are table-driven with `t.Parallel()` and use `testify/assert` (`require` for setup such as `NewSine` errors). Use `assert.InEpsilon`/`InDelta` for float comparisons.
- All code must be covered by tests, written with TDD in red -> green -> refactor cycles. Each cycle covers one behavior (e.g. validation, then `next`, then `nextSignedInt16`, then `Read`):
  1. Red: write the test(s) for that behavior first and run them. They must fail on an assertion, because the behavior is missing. A compile error is not red: first add a minimal stub (the signature returning zero values, e.g. `return nil, nil`) so the test compiles and fails. A test that passes before the implementation exists proves nothing; rework it.
  2. Green: write the simplest code that makes the failing tests pass, nothing they don't check. Run the whole suite (`make test`), not just the new tests.
  3. Refactor: improve names, duplication and comments in code and tests without changing behavior; the suite stays green after each step.

  Every change ships with a test that fails if the change is reverted.
- Interface methods (e.g. every `Oscillator` method: `Read`, `next`, `nextSignedInt16`) also need benchmarks. Use `for b.Loop()` (plus `b.SetBytes` for byte-oriented methods). All of them are currently 0 allocs/op for both oscillators; treat a new allocation as a regression.
- Test and benchmark names follow Go's naming of examples: `Test<Function>` or `Test<Type>_<Method>` (capitalized even for unexported identifiers, e.g. `TestSine_NextSignedInt16`), plus an optional `_<aspect>` starting with a lowercase letter (`TestNewSine_validation`, `TestSine_Read_encoding`). The underscore only separates these parts, so `-run TestSine_Read` runs every `Read` test.
- Validation tests assert the specific message (`assert.ErrorContains`) and keep the other params valid (e.g. 440 Hz / 44_100), because checks mask each other (a negative `samplingRate` also trips the Nyquist check).
- Use plain ASCII in code, comments, docs and messages: `PI`/`2PI` not `π`, `->` not `→`, `-` not `—`. Don't comment self-explanatory code; do explain DSP concepts (e.g. Nyquist).
- Branches are named `<issue>-<slug>` (e.g. `3-sine-wave`) and commit messages start with the GitHub issue number (`#3 Refactor ...`).
- Only the user commits and pushes. Never run `git commit` or `git push`; leave changes in the working tree.
- Work is tracked in `ISSUES.md`
