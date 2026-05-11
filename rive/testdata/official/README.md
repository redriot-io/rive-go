# Official Rive Runtime Test Assets

Downloaded from:
https://github.com/rive-app/rive-runtime/tree/main/tests/unit_tests/assets

License: MIT (same as rive-runtime)

## Purpose

Ground-truth validation of our .riv writer. We parse these files and compare
structural properties (ToC entries, object types, property values) against our
own output to ensure binary compatibility.

Conformance tests live in `rive/conformance_test.go`.

## Confirmed finding (2026-05-11)

Key 212 (`FileAssetContents.bytes`, `PropertyTypeBytes`) IS present in the
official ToC for all three font-embedded files, with **field-index=1**
(`PropertyTypeString` proxy).

Rationale: `PropertyTypeBytes` (value 4) cannot be represented in the 2-bit
ToC packing. The official rive-cpp encoder uses `PropertyTypeString` (1) as
the proxy because both types share identical wire encoding
(varuint length + payload). This lets forward-compat readers skip unknown
bytes blobs correctly without corrupting the object stream.

Our writer now matches this behavior (`writer.go`, ToC collection loop).

## Files

| File | Size | Description |
|------|------|-------------|
| `hello_world.riv` | 33KB | 1 Text, 1 TextStyle, 1 Run ("Hello World!"), embedded font |
| `new_text.riv` | 384KB | 5 Text, 13 TextStyles, 22 Runs (complex multi-style), embedded font |
| `ellipsis.riv` | 787KB | Text overflow with ellipsis, embedded font |
| `ball_test.riv` | 1KB | Basic shapes + state machine (no font) |
| `blend_test.riv` | 503B | Blend state machine (no font) |

## Update procedure

```bash
cd /path/to/rive-go
BASE=https://raw.githubusercontent.com/rive-app/rive-runtime/main/tests/unit_tests/assets
wget -q -O rive/testdata/official/hello_world.riv "$BASE/hello_world.riv"
wget -q -O rive/testdata/official/new_text.riv     "$BASE/new_text.riv"
wget -q -O rive/testdata/official/ellipsis.riv     "$BASE/ellipsis.riv"
wget -q -O rive/testdata/official/ball_test.riv    "$BASE/ball_test.riv"
wget -q -O rive/testdata/official/blend_test.riv   "$BASE/blend_test.riv"
```
