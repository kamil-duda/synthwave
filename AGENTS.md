# AGENTS.md

This file provides guidance to AI coding agents working with code in this repository.

## Commands

- `make run`: play the synth (opens the audio device and cycles sine -> square -> pulse -> sawtooth -> triangle every 2 s until killed)
- `make run-gc-flags`: run with `-gcflags="-m -m"` to inspect escape analysis / inlining
- `make test`: `go test -v ./...`
- `make bench`: benchmarks only (`-bench . -benchmem -run ^$`)
- `make coverage`: writes `coverage.html` and opens it
- `make vet`: `go vet ./...` static analysis
- `gofmt -l .`: must print nothing (no Makefile target)
- `make mod` / `make update`: `go mod tidy` / upgrade dependencies

Single test or benchmark:

```
go test -v -run 'TestOscillator_validation/sine/zero_frequency' ./oscillator
go test -v -bench BenchmarkName -benchmem -run '^$' ./oscillator
```

## Architecture

A Go audio synthesizer (module `synthwave`, Go 1.27) built on `github.com/ebitengine/oto/v3`.

Files:
- `main.go`: entry point. Creates a sine, a square, a pulse (`pulseWidth` 0.2), a sawtooth and a triangle oscillator (`triangleSymmetry` 0.2, because the classic 0.5 sounds almost like the sine) with the same `volume` (0.2, passed as their amplitude) and `baseFrequency` (440 Hz), the oto context and one player per oscillator. It plays the sine first and every `switchInterval` (2 s) pauses the active player and plays the next one, wrapping around. At the start and at every switch it logs the oscillator now playing with `log.Printf("playing %v", ...)` (time-stamped, to stderr), e.g. `playing pulse: 440 Hz, amplitude 0.2, pulseWidth 0.2, sampling rate 44100 Hz`. A paused player keeps its buffered samples, so each oscillator resumes where it stopped. The switch is not sample-accurate and can click. Meanwhile it polls every 10 ms, panics on an oto context error and exits when the active player stops playing. Audio settings are constants: `samplingRate` (44.1 kHz), `channels` (1, mono), `hardwareBufferSize` (50 ms OS buffer), `bufferSizeSamples` (oto player buffer; despite the name it is 4096 bytes, see below).
- `oscillator/oscillator.go`: the `Oscillator` interface and everything the oscillators share: `validate`, `phaseTracker`, `toSignedInt16`, `encode`, `describe` and `angularFrequency`.
- `oscillator/sine.go`: `NewSine` and the unexported `sine` struct.
- `oscillator/square.go`: `NewSquare` and the unexported `square` struct, a basic square wave: symmetric (50% duty cycle) and naive (not band-limited).
- `oscillator/pulse.go`: `NewPulse` and the unexported `pulse` struct, a naive pulse wave whose `pulseWidth` (duty cycle) is set in the constructor.
- `oscillator/sawtooth.go`: `NewSawtooth` and the unexported `sawtooth` struct, a naive rising sawtooth and the cheapest waveform (no `math.Sin`, no branch besides the phase wrap).
- `oscillator/triangle.go`: `NewTriangle` and the unexported `triangle` struct, a triangle wave whose `symmetry` (the part of the period it rises) is set in the constructor.
- `*_test.go` next to each file: tests (`main` has none; it needs an audio device). `oscillator_test.go` holds the `constructors` table with the tests shared by every oscillator.

oto uses a pull model: `main.go` creates an oto context and passes an oscillator to `otoCtx.NewPlayer`. The player then calls the oscillator's `Read([]byte)` whenever the audio buffer needs refilling. Each oscillator is therefore an `io.Reader` that encodes generated samples directly into the byte stream.

Data flow during playback: oto's player calls `Read(p)` with 4096-byte buffers (`player.SetBufferSize(bufferSizeSamples)` takes bytes, i.e. 2048 samples, ~46 ms) -> every oscillator's `Read` is `encode(p, next)` -> `encode` calls `next()` once per 2 bytes and scales it with `toSignedInt16` -> `next()` takes the current phase from `advance()` and returns the waveform's value at it. Oscillators never return `io.EOF`, so `make run` plays until killed.

**The sample format is an implicit contract between `main.go` and `Read`.** The oto context is set to 44.1 kHz, mono, `oto.FormatSignedInt16LE`. `encode` writes one little-endian int16 (2 bytes) per sample and does not interleave channels. Changing the channel count or sample format in `main.go` means changing `encode` too, or the audio will be garbled.

Oscillator pipeline (`oscillator/`). The shared parts live in `oscillator.go`; a waveform file holds only its struct (embedding `phaseTracker`), its constructor, `next()` and a one-line `Read`:
- Every constructor first calls `validate`, which checks the shared arguments in this order: amplitude in [0, 1] (NaN rejected), frequency (a `float64`, so notes like C4 = 261.63 Hz work) > 0 (NaN rejected), samplingRate > 0, then frequency below the Nyquist frequency (`samplingRate/2`, compared as floats so odd sampling rates work; +Inf fails here). `NewPulse` takes `pulseWidth` as its last argument and requires it in the open range (0, 1), because 0 or 1 would be a constant (silence). `NewTriangle` takes `symmetry` as its last argument and requires it in the closed range [0, 1] (NaN rejected), because 0 and 1 are not silence but a falling and a rising sawtooth. They return the interface rather than the concrete unexported struct.
- `next()` returns a float sample in `[-amplitude, amplitude]` at the phase returned by the embedded `phaseTracker.advance()`, which then adds `phaseStep = 2PIf / samplingRate` and wraps at 2PI with a single subtraction. That is enough only because the Nyquist check keeps `phaseStep < PI`. `sine` returns `amplitude * sin(phase)`. `square` returns `+amplitude` for phase in [0, PI) and `-amplitude` in [PI, 2PI). Its harmonics above the Nyquist frequency alias, and the check cannot prevent that (see the `square` doc comment). `pulse` returns `+amplitude` for phase in [0, 2PI*pulseWidth) and `-amplitude` for the rest (0.5 gives the square). It aliases like `square`, and any other `pulseWidth` gives it a DC offset (average `amplitude * (2*pulseWidth - 1)`), which is left for a DC-blocking filter (task #42 in `README.md`) because removing it in the oscillator would push the peak above `amplitude`. `sawtooth` returns `amplitude/PI * phase - amplitude`, a ramp from `-amplitude` towards `+amplitude` that jumps back at 2PI. It has every harmonic (1/n) and aliases more than `square`. `triangle` rises from `-amplitude` to `+amplitude` for phase in [0, 2PI*symmetry) and falls back for the rest, with both slopes precomputed. At symmetry 0 or 1 one part has zero length and an infinite slope that `next()` never uses. Neither has a DC offset.
- `toSignedInt16` scales that sample to int16 with `math.Round`, so values stay in [-32767, 32767].
- `encode` fills the buffer two bytes at a time, calls `next()` once per sample, returns `io.ErrShortBuffer` for `len(p) < 2` and leaves a trailing odd byte untouched. It calls `next` through a function value, which cannot be inlined (the `encode` comment says what that costs).

Errors: any non-EOF error from `Read` makes oto close the player, so `IsPlaying()` turns false and `main` exits without reporting it (it checks `otoCtx.Err()`, not `player.Err()`). An error in a paused player's `Read` shows up at the switch, because `Play()` does nothing on a player with an error.

The `Oscillator` interface includes the unexported method `next`, so new oscillator types must live in the `oscillator` package. It also embeds `fmt.Stringer`: every oscillator's `String()` is a one-liner calling `describe` with its name, amplitude, `phaseTracker` (which keeps `frequency` and `samplingRate` for this) and its own parameters, already formatted (`pulseWidth`, `symmetry`). A new oscillator also goes into the `constructors` table in `oscillator_test.go`.

Keep this section current: when a change adds or moves files, alters the data flow, the audio settings in `main.go`, the `Oscillator` interface or the validation rules, update this section in the same change. When an oscillator is added or its waveform changes, also update its ASCII diagram in `README.md`.

## Conventions

- Tests sit in the same package (white-box) so they can call unexported methods. They are table-driven with `t.Parallel()` and use `testify/assert` (`require` for setup such as `NewSine` errors). Use `assert.InEpsilon`/`InDelta` for float comparisons.
- All code must be covered by tests, written with TDD in red -> green -> refactor cycles. Each cycle covers one behavior (e.g. validation, then `next`, then `Read`):
  1. Red: write the test(s) for that behavior first and run them. They must fail on an assertion, because the behavior is missing. A compile error is not red: first add a minimal stub (the signature returning zero values, e.g. `return nil, nil`) so the test compiles and fails. A test that passes before the implementation exists proves nothing; rework it.
  2. Green: write the simplest code that makes the failing tests pass, nothing they don't check. Run the whole suite (`make test`), not just the new tests.
  3. Refactor: improve names, duplication and comments in code and tests without changing behavior; the suite stays green after each step.

  Every change ships with a test that fails if the change is reverted.
- Every oscillator is in the `constructors` table in `oscillator_test.go`, which runs the shared tests (validation, boundaries, `Read`) for it. Its own test file covers its shape (`Test<Type>_Next`), its description (`Test<Type>_String`), its own parameters and its benchmarks: `Benchmark<Type>_Next` (the waveform alone) and `Benchmark<Type>_Read` (a 4096-byte buffer, what oto requests, with `b.SetBytes`), both with `for b.Loop()`. `Read` runs at A4 440 Hz, C8 4186 Hz (the highest piano key) and just below the Nyquist frequency, because the frequency changes the speed: the higher it is, the more often the branches in `next()` and `advance()` change direction and the CPU predicts them worse (square is about a third slower just below the Nyquist frequency than at 440 Hz; sine varies without a clear trend, because `math.Sin` dominates its cost). Amplitude, `pulseWidth` and `symmetry` do not change the speed, so the benchmarks keep them fixed. The shared helpers are measured through them: `next()` runs `advance`, `Read` runs `encode` and `toSignedInt16`. All oscillators are 0 allocs/op; treat a new allocation as a regression.
- Test and benchmark names follow Go's naming of examples: `Test<Function>` or `Test<Type>_<Method>` (capitalized even for unexported identifiers, e.g. `TestPhaseTracker_Advance`), plus an optional `_<aspect>` starting with a lowercase letter (`TestNewPulse_validation`, `TestOscillator_Read_continuity`). The underscore only separates these parts, so `-run TestOscillator_Read` runs every `Read` test.
- Validation tests assert the specific message (`assert.ErrorContains`) and keep the other params valid (e.g. 440 Hz / 44_100), because checks mask each other (a negative `samplingRate` also trips the Nyquist check). The exception are rows that pin the order of checks and make two arguments invalid on purpose (e.g. `amplitude is checked before pulseWidth`).
- Use plain ASCII in code, comments, docs and messages: `PI`/`2PI` not `π`, `->` not `→`, `-` not `—`. Don't comment self-explanatory code; do explain DSP concepts (e.g. Nyquist).
- Branches are named `<task>-<slug>` (e.g. `3-sine-wave`) and commit messages start with the task number from `README.md` (`#3 Refactor ...`).
- Only the user commits and pushes. Never run `git commit` or `git push`; leave changes in the working tree.
- Work is tracked in the Tasks section of `README.md`, the only source of tasks. Don't use the `gh` CLI or the GitHub API (`gh api`, `gh issue`, ...) to browse the GitHub repo; everything needed is in `README.md`. Every task has a number; a new task gets the highest number in that section + 1.
