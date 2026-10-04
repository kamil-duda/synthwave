# synthwave

A small software synthesizer in Go, built on [oto](https://github.com/ebitengine/oto).

`make run` plays every oscillator in turn (2 s each), `make test` runs the tests and `make help` lists the other targets.
Planned work is in [TASKS.md](TASKS.md).

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
