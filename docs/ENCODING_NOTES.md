# .riv Encoding Notes — Debugging History & Gotchas

## Format overview (verified against ball_test.riv reference)

```
RIVE (4 bytes)
major (LEB128)   = 7
minor (LEB128)   = 0
fileID (LEB128)  = any

ToC: LEB128 property keys, terminated by 0
     Then 2-bit type fields packed into uint32s,
     4 keys per uint32 (only bits 0-7 used per word).
     Types: 0=uint/bool, 1=string, 2=float, 3=color.
     Number of words = ceil(numKeys / 4).

Object stream: typeKey(LEB128) + (propKey(LEB128) + value)* + 0
```

## Key format constraints

### ToC bit-packing (4 keys per uint32, NOT 16)

The C++ runtime reads one uint32 per 4 keys, advancing `currentBit` by 2 per key
and resetting to 0 (reading a new uint32) when `currentBit == 8`.  
Only bits 0–7 of each uint32 carry data.

For N ToC entries: **ceil(N / 4)** uint32s, each uint32 covers 4 entries.

Reference: `runtime_header.hpp` — the `currentBit` loop.

### Property values

| type | wire encoding              |
|------|---------------------------|
| uint/bool | LEB128 varuint      |
| string    | LEB128 len + UTF-8  |
| float     | 4-byte IEEE 754 LE  |
| color     | 4-byte uint32 LE (ARGB) |

Colors: `0xAARRGGBB` in code, stored little-endian on wire → `BB GG RR AA`.

### Object hierarchy

Required emission order:
1. Backboard (typeKey 23) — no properties needed
2. Artboard (typeKey 1) — name, width, height
3. Shape (typeKey 3) — name, parentId→artboard, x, y
4. Rectangle/Ellipse (typeKey 7/4) — parentId→shape, width, height
5. Fill (typeKey 20) — parentId→shape (NOT parentId→rectangle)
6. SolidColor (typeKey 18) — parentId→fill, colorValue
7. LinearAnimation (typeKey 31) — name, duration, loopValue
8. KeyedObject (typeKey 25) — objectId→target shape
9. KeyedProperty (typeKey 26) — propertyKey (e.g. 18=opacity)
10. KeyFrameDouble (typeKey 30) × N — frame, value, interpolationType

parentId is a **global object index** (Backboard=0, Artboard=1, etc.).

## Sentinel values — GO ZERO IS WRONG

Several fields use `^uint64(0)` (all-ones / 0xFFFFFFFFFFFFFFFF) as "not set".
Go's zero value (0) triggers emission because the condition is `!= sentinel`.

| Field                          | Key | Sentinel    | Go zero effect if not set   |
|-------------------------------|-----|-------------|------------------------------|
| LinearAnimation.WorkStart     | 60  | ^uint64(0)  | 0→work area starts at frame 0 |
| LinearAnimation.WorkEnd       | 61  | ^uint64(0)  | **0→work area ends at frame 0 → 0-duration → infinite loop** |
| InterpolatingKeyFrame.InterpolatorId | 69 | ^uint64(0) | 0→Backboard treated as cubic interpolator |
| Artboard.StyleId              | 494 | ^uint64(0) | 0→references wrong object |
| Artboard.DefaultStateMachineId | 236 | ^uint64(0) | 0→wrong SM |
| Artboard.ViewModelId          | 583 | ^uint64(0) | 0→wrong VM |

**Rule:** whenever gen_*.go says `if o.Field != ^uint64(0) { emit }`, the builder MUST
set that field to `^uint64(0)` if you don't intend to reference a specific object.

## Root cause of infinite render loop (T-365)

`LinearAnimation.WorkEnd = 0` was emitted in the binary (Go zero, uncorrected sentinel).

The Rive WASM runtime computes animation duration using `workEnd`. With `workEnd=0`,
the effective animation duration becomes 0 frames. The loop-advance code
(`while (time >= endSeconds)`) runs with `endSeconds=0` and `durationSeconds=0`,
causing it to subtract 0 from time on every iteration — **infinite busy loop in WASM**.

Fix applied in `rive/builder/anim.go`:
```go
la.WorkStart = ^uint64(0)  // suppress emission
la.WorkEnd   = ^uint64(0)  // suppress emission
```

Similarly `InterpolatorId` on every keyframe was emitting 0 (Backboard as
cubic interpolator). Fixed by setting `f.InterpolatorId = ^uint64(0)`.

## Default vs sentinel fields

Not all non-zero defaults use the sentinel pattern. Some use explicit values:

| Field                              | Emit condition      | Default |
|------------------------------------|---------------------|---------|
| LinearAnimation.Fps                | `!= 60`             | 60      |
| LinearAnimation.Duration           | `!= 60`             | 60 (frames) |
| LinearAnimation.Speed              | `!= 1`              | 1.0     |
| LinearAnimation.LoopValue          | `!= 0`              | 0 (OneShot) |
| ShapePaint.IsVisible               | `!o.IsVisible`      | true    |
| ShapePaint.BlendModeValue          | `!= 127`            | 127     |
| Node.Opacity                       | `!= 1`              | 1.0     |
| Node.ScaleX/Y                      | `!= 1`              | 1.0     |
| ParametricPath.OriginX/Y           | `!= 0.5`            | 0.5     |
| Rectangle.LinkCornerRadius         | `!o.LinkCornerRadius` | true  |
| Stroke.Thickness                   | `!= 1`              | 1.0     |

See `02_Research/PROJ_RIVE-GO/defaults-map.json` for the full map.

## Minimal static .riv structure (96 bytes)

```
Artboard "Test" 500×500
  Shape "rect" at (200, 200)
    Rectangle 100×100
    Fill → SolidColor #FF0000
```

Object stream has no animation objects. Used to isolate format from animation bugs.
Generated by `cmd/examples/minimal.go` → `docs/preview/examples/minimal_static.riv`.

## ToC type correctness

The ToC type for a property key only matters for **skipping unknown properties**.
For known object types, the C++ runtime always reads properties using hardcoded types
(from generated `*_base.hpp`). A ToC type mismatch for a known key has no rendering effect.

The ToC only needs to be correct for property keys that may appear in object types
the current runtime version doesn't know. Listing known keys in the ToC is safe but
redundant — the runtime ignores ToC for keys it already knows.
