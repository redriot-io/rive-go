# .riv Format Contract
## Derived from official rive-runtime test assets (2026-05-11)
## Source: https://github.com/rive-app/rive-runtime/tree/main/tests/unit_tests/assets

This document describes the binary encoding rules observed from official Rive editor exports.
A conformant writer can be implemented from this specification without reading any source code.

---

## 1. File Structure

```
[Fingerprint: 4 bytes "RIVE"]
[Header: varuint major, varuint minor, varuint fileID]
[Table of Contents]
[Object Stream]
```

---

## 2. Table of Contents (ToC) Encoding

The ToC provides type hints for forward-compatibility (allowing clients to skip unknown
properties they don't recognise). It does NOT include all properties — only those needed
for correct skipping by unknown clients.

### 2.1 ToC Structure
```
[varuint key₁] [varuint key₂] ... [varuint key_N] [varuint 0]  ← key list, terminated by 0
[uint32 word₀] [uint32 word₁] ...                               ← 2-bit field indices, 4 per word
```

### 2.2 Field Index (2-bit) Values
| FieldIdx | Wire Encoding | Go Name |
|----------|--------------|---------|
| 0 | LEB128 varuint (unsigned) | PropertyTypeUint |
| 1 | varuint_length + payload (string OR bytes) | PropertyTypeString |
| 2 | 4 bytes little-endian IEEE 754 float32 | PropertyTypeFloat |
| 3 | 4 bytes little-endian uint32 (ARGB) | PropertyTypeColor |

### 2.3 Critical: Bytes Properties in ToC
`PropertyTypeBytes` (canonical value 4) **cannot** be represented in the 2-bit field index.
The official encoder emits bytes properties with **field-index=1** (the string proxy).
This is safe because string and bytes share identical wire encoding (varuint_length + raw payload).
Forward-compat readers can skip the blob correctly using field-index=1.

**Confirmed bytes properties observed in official ToC with field-index=1:**
- Key 212 (`FileAssetContents.bytes`) — present in all font-embedded .riv files

**Wrong approach:** field-index=0 (uint). The runtime would read the LEB128 length as a uint value,
then the 80KB blob would corrupt the rest of the object stream.

**Our reader fix:** `lookupPropType` prefers `globalPropTypes` (compiled-in) over ToC values,
so our reader correctly handles both old (broken) and new (correct) files.

### 2.4 ToC Minimality
The official encoder includes in the ToC only property keys that:
1. Are NOT in the runtime's compiled-in CoreRegistry (forward-compat unknown properties)
2. OR are bytes-typed (need the field-index=1 proxy for correct skipping)

Properties like name (4), x (13), y (14), width (7), height (8) are typically absent from the
ToC because the runtime knows their types from CoreRegistry.

**Observed ToC key sets by file type:**

`ball_test.riv` (no fonts, 8 ToC keys):
artboardId(197), animationId(198), targetId(224), listenerTypeValue(225),
defaultStateMachineId(236), k389(389), k399(399), editModeValue(494)

`hello_world.riv` (1 font, 7 ToC keys):
assetId(204), bytes(212,field-idx=1), defaultStateMachineId(236),
text(268,field-idx=1), styleId(272), fontSize(274), fontAssetId(279)

`new_text.riv` (5 fonts, 12 ToC keys):
assetId(204), bytes(212,field-idx=1), defaultStateMachineId(236),
text(268,field-idx=1), styleId(272), fontSize(274), fontAssetId(279),
value(280,field-idx=1), sizingValue(284), width(285), height(286), k287(287)

---

## 3. Object Stream Encoding

Each object:
```
[varuint typeKey]
[varuint propKey₁] [encoded value₁]
[varuint propKey₂] [encoded value₂]
...
[varuint 0]   ← property terminator
```

Wire encoding by property type:
- **uint**: LEB128 varuint (unsigned)
- **string**: varuint_length + UTF-8 bytes
- **bytes**: varuint_length + raw bytes (same wire format as string)
- **float**: 4 bytes little-endian IEEE 754 float32
- **color**: 4 bytes little-endian uint32 (ARGB: 0xAARRGGBB)

### 3.1 Default Omission
Properties at their default value are omitted from the object stream.

Common defaults (property omitted when equal to):
| Key | Property | Default |
|-----|----------|---------|
| 13 | x | 0.0 |
| 14 | y | 0.0 |
| 18 | opacity | 1.0 |
| 23 | blendModeValue | 3 (normal) |
| 37 | colorValue | 0xFF000000 |
| 59 | loopValue | 0 (oneShot) |
| 274 | fontSize | 0.0 (not 12.0 — emit always for text) |
| 703 | fitFromBaseline | true (omit) |
| 932 | textRunListSource | ^uint64(0) sentinel (omit) |

---

## 4. Object Emission Order Patterns

### 4.1 Font-Embedded Text Files (observed in hello_world.riv, new_text.riv, ellipsis.riv)

```
[0] Backboard (typeKey=23)
[1..N] FontAsset (typeKey=141) + FileAssetContents (typeKey=106) pairs  ← BEFORE Artboard
[N+1] Artboard (typeKey=1)
  [N+2] Text (typeKey=134)                         ← parentId=0 → Artboard
    [N+3] TextStyle (typeKey=137)                  ← parentId=1 → Text
      [N+4] Fill (typeKey=20)                      ← parentId=2 → TextStyle
        [N+5] SolidColor (typeKey=18)              ← parentId=? → Fill
    [N+6] TextValueRun (typeKey=135)               ← parentId=1 → Text
```

**CRITICAL: Fonts emit BEFORE the Artboard** — FontAsset and FileAssetContents pairs appear
between Backboard and the first Artboard. Our builder currently emits them after the Artboard.

### 4.2 Shape with Fill

```
Artboard → Shape → [path type] → Fill → SolidColor
```

### 4.3 State Machine

```
→ LinearAnimation(s) → StateMachine → StateMachineLayer → AnimationState/AnyState/EntryState/ExitState → StateTransition(s)
```

---

## 5. Type Key Reference Table

| typeKey | Name | Description |
|---------|------|-------------|
| 1 | Artboard | Root canvas with width/height |
| 2 | Node | Transform node |
| 3 | Shape | Visible shape container |
| 4 | Ellipse | Ellipse path |
| 5 | StraightVertex | Polygon vertex |
| 6 | CubicDetachedVertex | Bezier vertex (detached handles) |
| 7 | Rectangle | Rectangle path |
| 9 | CubicMirroredVertex | Bezier vertex (mirrored handles) |
| 10 | CubicAsymmetricVertex | Bezier vertex (asymmetric handles) |
| 16 | PointsPath | Custom path (from vertices) |
| 17 | RadialGradient | Radial gradient paint |
| 18 | SolidColor | Solid color (child of Fill/Stroke) |
| 19 | GradientStop | Gradient color stop |
| 20 | Fill | Fill paint container |
| 22 | LinearGradient | Linear gradient paint |
| 23 | Backboard | Root document object (always [0], no parentId) |
| 24 | Stroke | Stroke paint container |
| 25 | KeyedObject | Animation target reference |
| 26 | KeyedProperty | Animated property reference |
| 28 | CubicEaseInterpolator | Custom cubic bezier easing |
| 30 | KeyFrameDouble | Float value keyframe |
| 31 | LinearAnimation | Named animation clip |
| 37 | KeyFrameColor | Color value keyframe |
| 48 | DrawTarget | Draw order target |
| 49 | DrawRules | Draw order rules |
| 50 | KeyFrameId | Id/uint value keyframe |
| 53 | StateMachine | State machine definition |
| 55 | StateMachineInput | Base SM input |
| 56 | StateMachineNumber | Numeric SM input |
| 57 | StateMachineLayer | SM layer |
| 59 | StateMachineBool | Boolean SM input |
| 61 | AnimationState | SM state linked to animation |
| 62 | AnyState | SM any-state |
| 63 | EntryState | SM entry state |
| 64 | ExitState | SM exit state |
| 65 | StateTransition | SM transition |
| 67 | TransitionInputCondition | SM transition condition |
| 92 | NestedArtboard | Nested artboard reference |
| 99 | Asset | Base asset |
| 103 | FileAsset | Base file asset |
| 106 | FileAssetContents | Embedded asset bytes |
| 128 | Event | Named event |
| 134 | Text | Text object |
| 135 | TextValueRun | Text run (content + style reference) |
| 137 | TextStyle | Text style (font, size) |
| 141 | FontAsset | Font asset metadata |
| 168 | NestedTrigger | Nested artboard trigger |
| 171 | BlendAnimationDirect | Direct blend animation |
| 420 | ArtboardComponentList | Artboard component list |
| 573 | TextStyleFeature | Text style feature (OpenType feature) |

---

## 6. Property Key Reference Table (Common)

| Key | Name | Wire Type | Notes |
|-----|------|-----------|-------|
| 3 | dependentIds | bytes | |
| 4 | name | string | Component name |
| 5 | parentId | uint | **Artboard-relative index** (see §7) |
| 6 | childOrder | uint | |
| 7 | width | float | Artboard/shape width |
| 8 | height | float | Artboard/shape height |
| 9 | xArtboard | float | Artboard X offset (editor) |
| 10 | yArtboard | float | Artboard Y offset (editor) |
| 11 | originX | float | |
| 12 | originY | float | |
| 13 | x | float | Component X position |
| 14 | y | float | Component Y position |
| 15 | rotation | float | Rotation (radians) |
| 16 | scaleX | float | |
| 17 | scaleY | float | |
| 18 | opacity | float | 0.0–1.0 |
| 20 | width | float | Shape width |
| 21 | height | float | Shape height |
| 23 | blendModeValue | uint | 3=Normal |
| 26 | radius | float | |
| 37 | colorValue | color | 0xAARRGGBB |
| 40 | fillRule | uint | |
| 41 | isVisible | uint | 0=false, 1=true |
| 47 | thickness | float | Stroke width |
| 51 | objectId | uint | |
| 52 | animationId | uint | |
| 53 | propertyKey | uint | |
| 55 | name | string | Animation/SM name |
| 56 | fps | uint | |
| 57 | duration | uint | Frames |
| 58 | speed | float | |
| 59 | loopValue | uint | 0=oneShot, 1=loop, 2=pingPong |
| 67 | frame | uint | Keyframe position |
| 68 | interpolationType | uint | |
| 70 | value | float | Keyframe value |
| 88 | value | color | Keyframe color value |
| 119 | drawableId | uint | |
| 121 | drawTargetId | uint | |
| 126 | cornerRadius | float | |
| 127 | innerRadius | float | |
| 138 | name | string | Layer/input name |
| 149 | animationId | uint | |
| 151 | stateToId | uint | |
| 155 | inputId | uint | |
| 156 | opValue | uint | |
| 157 | value | float | |
| 166 | value | float | |
| 170 | blendStateId | uint | |
| 172 | strength | float | |
| 197 | artboardId | uint | |
| 198 | animationId | uint | |
| 203 | name | string | FontAsset name |
| 204 | assetId | uint | FontAsset CDN ID |
| 212 | bytes | bytes | FileAssetContents embedded bytes |
| 223 | triangleIndexBytes | bytes | Mesh triangle indices |
| 236 | defaultStateMachineId | uint | |
| 268 | text | string | TextValueRun content |
| 272 | styleId | uint | TextValueRun → TextStyle artboard-relative index |
| 274 | fontSize | float | |
| 279 | fontAssetId | uint | TextStyle → FontAsset artboard-relative index |
| 281 | alignValue | uint | 0=left, 1=right, 2=center |
| 284 | sizingValue | uint | |
| 285 | width | float | Text box width |
| 286 | height | float | Text box height |
| 359 | cdnUuid | bytes | FontAsset CDN UUID |
| 362 | cdnBaseUrl | string | FontAsset CDN base URL |
| 370 | lineHeight | float | -1.0 = auto |
| 390 | letterSpacing | float | |
| 395 | frame | uint | |
| 494 | editModeValue | uint | Editor-only |
| 703 | fitFromBaseline | uint | Default=true (emit to suppress) |
| 932 | textRunListSource | uint | Sentinel=^uint64(0) (emit to suppress) |

---

## 7. ParentId Rules

`parentId` (key 5) uses **artboard-relative** indices, not global stream indices.

### 7.1 Resolution
```
global_index = artboard_global_index + parentId_value
```
where `artboard_global_index` is the global stream index of the owning Artboard.
`parentId=0` always refers to the Artboard itself.

### 7.2 Hierarchy Rules (from official files)
- **Backboard** (global [0]): no parentId property. Implicit document root.
- **Artboard** (typeKey=1): no parentId property. Multiple artboards in one file are all siblings.
- **FontAsset** (typeKey=141): emitted before its Artboard; no parentId in official files.
- **FileAssetContents** (typeKey=106): emitted immediately after its FontAsset; no parentId.
- **Text** (typeKey=134): parentId=0 → Artboard
- **TextStyle** (typeKey=137): parentId=N → its parent Text
- **Fill** (typeKey=20): parentId=N → TextStyle (for text) or Shape (for shapes)
- **SolidColor** (typeKey=18): parentId=N → its parent Fill
- **TextValueRun** (typeKey=135): parentId=N → its parent Text
- **styleId** (272): artboard-relative index of the TextStyle
- **fontAssetId** (279): artboard-relative index of the FontAsset

### 7.3 Known Discrepancy (our builder vs official)
Our builder computes artboard-relative indices from `global_index - artboardOffset`
where artboardOffset = Artboard's global index. This is correct for objects emitted
after the Artboard. However, if FontAssets are emitted before the Artboard (as in
official files), their artboard-relative indices require special handling.

---

## 8. Observed Emission Order Differences (Our Writer vs Official)

| Aspect | Official Rive Editor | Our Builder |
|--------|---------------------|-------------|
| Font location | FontAsset/FAC pairs BEFORE Artboard | Fonts AFTER Artboard |
| TextStyle typeKey | 137 | 573 (TextStyleFeature — incorrect!) |
| ToC for common props | Excluded (runtime knows from CoreRegistry) | Included |
| Fill order | SolidColor → TextValueRun → Fill | Fill → SolidColor within style |

**Action items for conformance:**
1. Move font emission before Artboard in builder
2. Fix TextStyle typeKey from 573 to 137
3. Review whether Fill should come before or after SolidColor+TextValueRun

---

## 9. Round-Trip Notes

Reading an official file with our reader and writing it back via our writer will produce
a structurally equivalent but not byte-identical file due to:
- ToC key ordering differences (we sort by first-occurrence in stream)
- Property value differences (writer uses Go zero-values for some defaults)
- The above emission order differences

The correct conformance check is: semantic equivalence of object graphs
(same typeKeys, same property keys+values) rather than byte equality.

---

## 10. Version Compatibility

All observed official files use Major=7, Minor=0.
The major version determines the decoder algorithm; minor is informational.
FileID is an arbitrary uint64 (0 for our generated files, non-zero for editor exports).
