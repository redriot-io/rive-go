// rivtool inspects, validates, and creates Rive (.riv) files.
//
// Usage:
//
//	rivtool inspect <file.riv>                — dump object tree with properties
//	rivtool validate <file.riv>               — check well-formedness of a .riv
//	rivtool validate --schema <scene.json>    — validate a JSON scene file
//	rivtool verify <file.riv>                 — structural wiring checks (SM, listeners, refs)
//	rivtool create --from <scene.json>        — build .riv from JSON (stdout)
//	rivtool create --from <scene.json> --output <out.riv>
//	rivtool create --from -                   — read JSON from stdin
//	rivtool generate                          — regenerate docs/preview/examples/
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "inspect":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "inspect requires a file argument")
			os.Exit(1)
		}
		cmdInspect(os.Args[2])
	case "validate":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "validate requires a file argument")
			os.Exit(1)
		}
		// validate --schema <file.json> — JSON scene validation
		if os.Args[2] == "--schema" {
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "validate --schema requires a JSON file argument")
				os.Exit(1)
			}
			cmdValidateSchema(os.Args[3])
			return
		}
		ok := cmdValidate(os.Args[2])
		if !ok {
			os.Exit(1)
		}
	case "create":
		cmdCreate(os.Args[2:])
	case "verify":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "verify requires a file argument")
			os.Exit(1)
		}
		ok := cmdVerify(os.Args[2])
		if !ok {
			os.Exit(1)
		}
	case "generate":
		cmdGenerate()
	case "dump":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "dump requires a file argument")
			os.Exit(1)
		}
		cmdDump(os.Args[2])
	case "analyze":
		cmdAnalyze(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`rivtool — rive-go inspector, validator, and creator

Commands:
  inspect  <file.riv>                    dump object tree (typeKey, properties)
  validate <file.riv>                    check well-formedness of a .riv file
  validate --schema <scene.json>         validate a JSON scene file (no build)
  verify   <file.riv>                    structural wiring checks (SM, listeners, refs)
  create   --from <scene.json>           build .riv from JSON scene, write to stdout
  create   --from <scene.json> --output <out.riv>
  create   --from -                      read JSON from stdin
  dump     <file.riv>                    full structural decoder with parentId resolution
  analyze  --assets <dir> [--defs <dir>] [-o <file.json>]
                                         extract format_contract.json from .riv assets + dev/defs
  generate                               regenerate docs/preview/examples/

JSON Scene format (version 1):
  {
    "version": 1,
    "artboard": {
      "name": "Main", "width": 400, "height": 400,
      "children": [
        { "type": "rectangle", "name": "box", "x": 200, "y": 200,
          "width": 100, "height": 100, "fill": "#FF0000" }
      ],
      "animations": [...],
      "state_machines": [...]
    }
  }`)
}

// ── create ────────────────────────────────────────────────────────────────────

// cmdDump implements: rivtool dump <file.riv>
// Produces a complete, human-readable structural analysis of a .riv file.
func cmdDump(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}

	f, err := rive.ReadBytes(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
		os.Exit(1)
	}

	// ── Header ────────────────────────────────────────────────────────────────
	fmt.Println("=== HEADER ===")
	fmt.Printf("Fingerprint: RIVE\n")
	fmt.Printf("Major: %d  Minor: %d  FileID: %d\n", f.MajorVersion, f.MinorVersion, f.FileID)
	fmt.Printf("File size: %d bytes\n", len(raw))

	// ── ToC ───────────────────────────────────────────────────────────────────
	toc := f.TocEntries()
	tocKeys := make([]uint32, 0, len(toc))
	for k := range toc {
		tocKeys = append(tocKeys, k)
	}
	sortUint32(tocKeys)

	wireTypeName := func(ft rive.PropertyType) string {
		switch ft {
		case 0:
			return "uint"
		case 1:
			return "string/bytes"
		case 2:
			return "float"
		case 3:
			return "color"
		default:
			return fmt.Sprintf("unknown(%d)", ft)
		}
	}

	fmt.Printf("\n=== TABLE OF CONTENTS (%d keys) ===\n", len(tocKeys))
	fmt.Printf("%-6s  %-8s  %s\n", "Key", "FieldIdx", "WireType")
	for _, k := range tocKeys {
		name := propName(k)
		ft := toc[k]
		canonical, _ := rive.LookupGlobalPropType(k)
		note := ""
		if canonical == rive.PropertyTypeBytes && ft == 1 {
			note = "  ← bytes proxied as string/field-idx=1"
		}
		fmt.Printf("%-6d  %-8d  %-14s  %s%s\n", k, ft, wireTypeName(ft), name, note)
	}

	// ── Object stream ──────────────────────────────────────────────────────────
	fmt.Printf("\n=== OBJECT STREAM (%d objects) ===\n", len(f.Objects))

	// Compute artboard offsets: parentId values are artboard-relative.
	// Track the global index of the most recently emitted Artboard as the offset.
	artboardOffsets := make([]int, len(f.Objects)) // artboardOffsets[i] = global index of owning Artboard
	currentArtboardIdx := -1
	for i, o := range f.Objects {
		if o.TypeKey() == 1 { // Artboard
			currentArtboardIdx = i
		}
		artboardOffsets[i] = currentArtboardIdx
	}

	resolveParentId := func(objIdx int, parentId uint64) string {
		ao := artboardOffsets[objIdx]
		if ao < 0 {
			return fmt.Sprintf("%d (no artboard context)", parentId)
		}
		// artboard-relative parentId 0 = the Artboard itself
		globalIdx := ao + int(parentId)
		if globalIdx >= 0 && globalIdx < len(f.Objects) {
			tname := dumpTypeKeyName(f.Objects[globalIdx].TypeKey())
			// Try to find name property
			for _, p := range f.Objects[globalIdx].Properties() {
				if p.Key == 4 || p.Key == 55 {
					tname += fmt.Sprintf(" %q", p.Value.(string))
					break
				}
			}
			return fmt.Sprintf("%d → [%d] %s", parentId, globalIdx, tname)
		}
		return fmt.Sprintf("%d (out of range)", parentId)
	}

	// Build parentMap for depth calculation (use artboard-relative offsets)
	parentMap := make(map[int]int)
	for i, o := range f.Objects {
		parentMap[i] = -1
		ao := artboardOffsets[i]
		for _, p := range o.Properties() {
			if p.Key == 5 && ao >= 0 {
				parentMap[i] = ao + int(p.Value.(uint64))
			}
		}
	}

	for i, o := range f.Objects {
		tname := dumpTypeKeyName(o.TypeKey())
		fmt.Printf("\n[%d] %s (typeKey=%d)\n", i, tname, o.TypeKey())
		props := o.Properties()
		if len(props) == 0 {
			fmt.Printf("    (no properties)\n")
			continue
		}
		for _, p := range props {
			name := propName(p.Key)
			var val string
			switch p.Type {
			case rive.PropertyTypeUint:
				uv := p.Value.(uint64)
				if p.Key == 5 { // parentId — resolve artboard-relative
					fmt.Printf("    parentId (%d) = %s [uint]\n", p.Key, resolveParentId(i, uv))
					continue
				}
				val = fmt.Sprintf("%d", uv)
			case rive.PropertyTypeString:
				sv := p.Value.(string)
				if len(sv) > 80 {
					sv = sv[:80] + "…"
				}
				val = fmt.Sprintf("%q", sv)
			case rive.PropertyTypeFloat:
				val = fmt.Sprintf("%g", p.Value.(float64))
			case rive.PropertyTypeColor:
				val = fmt.Sprintf("0x%08X", uint32(p.Value.(uint64)))
			case rive.PropertyTypeBytes:
				b := p.Value.([]byte)
				val = fmt.Sprintf("<%d bytes>", len(b))
			default:
				val = fmt.Sprintf("?(%v)", p.Value)
			}
			fmt.Printf("    %s (%d) = %s [%s]\n", name, p.Key, val, canonicalWireType(p.Type))
		}
	}

	// ── Summary ────────────────────────────────────────────────────────────────
	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Objects: %d\n", len(f.Objects))

	typeCounts := make(map[uint32]int)
	for _, o := range f.Objects {
		typeCounts[o.TypeKey()]++
	}
	typeKeys := make([]uint32, 0, len(typeCounts))
	for k := range typeCounts {
		typeKeys = append(typeKeys, k)
	}
	sortUint32(typeKeys)
	var typeDist []string
	for _, tk := range typeKeys {
		typeDist = append(typeDist, fmt.Sprintf("%s(%d)", dumpTypeKeyName(tk), typeCounts[tk]))
	}
	fmt.Printf("Type distribution: %s\n", strings.Join(typeDist, " "))
	fmt.Printf("ToC keys: %d\n", len(tocKeys))

	// List bytes properties (canonical type 4) present in ToC
	var bytesInToc []string
	for _, k := range tocKeys {
		if canonical, ok := rive.LookupGlobalPropType(k); ok && canonical == rive.PropertyTypeBytes {
			bytesInToc = append(bytesInToc, fmt.Sprintf("key %d %s (field_idx=%d)", k, propName(k), toc[k]))
		}
	}
	if len(bytesInToc) > 0 {
		fmt.Printf("Bytes properties in ToC: %s\n", strings.Join(bytesInToc, ", "))
	} else {
		fmt.Printf("Bytes properties in ToC: none\n")
	}

	// Tree depth
	maxDepth := 0
	for i := range f.Objects {
		d := depthOf(i, parentMap, 0)
		if d > maxDepth {
			maxDepth = d
		}
	}
	fmt.Printf("ParentId tree depth: %d\n", maxDepth)

	totalBytes := 0
	for _, o := range f.Objects {
		for _, p := range o.Properties() {
			if p.Type == rive.PropertyTypeBytes {
				totalBytes += len(p.Value.([]byte))
			}
		}
	}
	if totalBytes > 0 {
		fmt.Printf("Embedded bytes total: %d bytes\n", totalBytes)
	}
}

func sortUint32(s []uint32) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func canonicalWireType(pt rive.PropertyType) string {
	switch pt {
	case 0:
		return "uint"
	case 1:
		return "string"
	case 2:
		return "float"
	case 3:
		return "color"
	case 4:
		return "bytes"
	default:
		return "unknown"
	}
}

func propName(key uint32) string {
	names := map[uint32]string{
		3: "dependentIds", 4: "name", 5: "parentId", 6: "childOrder",
		7: "width", 8: "height", 9: "xArtboard", 10: "yArtboard",
		11: "originX", 12: "originY", 13: "x", 14: "y",
		15: "rotation", 16: "scaleX", 17: "scaleY", 18: "opacity",
		20: "width", 21: "height", 23: "blendModeValue", 24: "x", 25: "y",
		26: "radius", 31: "cornerRadiusTL", 32: "isClosed",
		33: "startY", 34: "endX", 35: "endY",
		37: "colorValue", 38: "colorValue", 39: "position", 40: "fillRule",
		41: "isVisible", 42: "startX", 43: "activeArtboardId", 44: "mainArtboardId",
		45: "colorValue", 46: "opacity", 47: "thickness", 48: "cap",
		49: "join", 50: "transformAffectsStroke", 51: "objectId", 52: "animationId",
		53: "propertyKey", 54: "artboardId", 55: "name", 56: "fps",
		57: "duration", 58: "speed", 59: "loopValue", 60: "workStart",
		61: "workEnd", 62: "enableWorkArea", 63: "x1", 64: "y1",
		65: "x2", 66: "y2", 67: "frame", 68: "interpolationType",
		69: "interpolatorId", 70: "value", 71: "keyedObjectId", 72: "keyedPropertyId",
		73: "order", 79: "rotation", 88: "value",
		92: "sourceId", 114: "start", 115: "end", 116: "offset",
		119: "drawableId", 120: "placementValue", 121: "drawTargetId",
		123: "originX", 124: "originY", 125: "points", 126: "cornerRadius",
		127: "innerRadius", 130: "flags", 137: "machineId", 138: "name",
		139: "machineOrder", 140: "value", 141: "value", 149: "animationId",
		150: "stateFromId", 151: "stateToId", 152: "flags",
		153: "transitionId", 154: "conditionOrder", 155: "inputId",
		156: "opValue", 157: "value", 158: "duration", 159: "transitionOrder",
		160: "exitTime", 161: "cornerRadiusTR", 162: "cornerRadiusBL", 163: "cornerRadiusBR",
		164: "linkCornerRadius", 165: "animationId", 166: "value", 167: "inputId",
		168: "inputId", 169: "animationOrder", 170: "blendStateId",
		171: "exitBlendAnimationId", 172: "strength", 173: "targetId",
		176: "styleValue", 197: "artboardId", 198: "animationId",
		199: "speed", 200: "mix", 201: "isPlaying", 202: "time",
		203: "name", 204: "assetId", 205: "order", 206: "assetId",
		207: "height", 208: "width", 209: "parentId", 211: "size",
		212: "bytes", 213: "assetId", 215: "u", 216: "v", 217: "isClosed",
		220: "toId", 223: "triangleIndexBytes", 224: "targetId",
		225: "listenerTypeValue", 226: "listenerId", 227: "inputId",
		228: "value", 229: "value", 230: "order", 231: "playbackValue",
		232: "playbackValue", 233: "layer", 234: "x", 235: "y",
		236: "defaultStateMachineId", 237: "inputId", 238: "nestedValue",
		239: "nestedValue", 240: "targetId",
		248: "url", 268: "text", 272: "styleId", 274: "fontSize",
		279: "fontAssetId", 280: "value", 281: "alignValue", 284: "sizingValue",
		285: "width", 286: "height", 359: "cdnUuid", 362: "cdnBaseUrl",
		370: "lineHeight", 390: "letterSpacing", 395: "frame",
		494: "editModeValue", 703: "fitFromBaseline", 932: "textRunListSource",
	}
	if n, ok := names[key]; ok {
		return n
	}
	return fmt.Sprintf("k%d", key)
}

func dumpTypeKeyName(k uint32) string {
	names := map[uint32]string{
		1: "Artboard", 2: "Node", 3: "Shape", 4: "Ellipse",
		5: "StraightVertex", 6: "CubicDetachedVertex", 7: "Rectangle",
		8: "Triangle", 9: "CubicMirroredVertex", 10: "CubicAsymmetricVertex",
		16: "PointsPath", 17: "RadialGradient", 18: "SolidColor",
		19: "GradientStop", 20: "Fill", 21: "ShapePaint",
		22: "LinearGradient", 23: "Backboard", 24: "Stroke",
		25: "KeyedObject", 26: "KeyedProperty", 28: "CubicEaseInterpolator",
		30: "KeyFrameDouble", 31: "LinearAnimation", 37: "KeyFrameColor",
		38: "WorldTransformComponent", 42: "ClippingShape", 47: "TrimPath",
		48: "DrawTarget", 49: "DrawRules", 50: "KeyFrameId",
		51: "Polygon", 52: "Star", 53: "StateMachine",
		55: "StateMachineInput", 56: "StateMachineNumber", 57: "StateMachineLayer",
		58: "StateMachineTrigger", 59: "StateMachineBool",
		61: "AnimationState", 62: "AnyState", 63: "EntryState",
		64: "ExitState", 65: "StateTransition", 67: "TransitionInputCondition",
		68: "TransitionTriggerCondition", 70: "TransitionNumberCondition",
		71: "TransitionBoolCondition", 84: "KeyFrameBool",
		91: "WorldTransformComponent", 92: "NestedArtboard",
		95: "NestedLinearAnimation", 99: "Asset", 100: "Image",
		103: "FileAsset", 106: "FileAssetContents", 109: "Mesh",
		114: "NestedStateMachine", 128: "Event", 134: "Text",
		135: "TextValueRun", 137: "TextStyle", 138: "TextModifierGroup",
		141: "FontAsset", 168: "NestedTrigger", 170: "InterpolatingKeyFrame",
		171: "BlendAnimationDirect", 420: "ArtboardComponentList",
		450: "KeyFrameUint", 573: "TextStyleFeature",
	}
	if n, ok := names[k]; ok {
		return n
	}
	return fmt.Sprintf("Unknown(typeKey=%d)", k)
}

// cmdCreate implements: rivtool create --from <file|--> [--output <out.riv>]
func cmdCreate(args []string) {
	var fromPath, outputPath string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from", "-f":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "create: --from requires a file path or -")
				os.Exit(1)
			}
			i++
			fromPath = args[i]
		case "--output", "-o":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "create: --output requires a file path")
				os.Exit(1)
			}
			i++
			outputPath = args[i]
		default:
			fmt.Fprintf(os.Stderr, "create: unknown flag %q\n", args[i])
			fmt.Fprintln(os.Stderr, "Usage: rivtool create --from <scene.json> [--output <out.riv>]")
			os.Exit(1)
		}
	}

	if fromPath == "" {
		fmt.Fprintln(os.Stderr, "create: --from is required")
		fmt.Fprintln(os.Stderr, "Usage: rivtool create --from <scene.json> [--output <out.riv>]")
		os.Exit(1)
	}

	// Read input
	var data []byte
	var err error
	if fromPath == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create: read stdin: %v\n", err)
			os.Exit(1)
		}
	} else {
		data, err = os.ReadFile(fromPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create: read %s: %v\n", fromPath, err)
			os.Exit(1)
		}
	}

	// Parse and build — use FromJSONFile when reading from a file so font
	// relative paths are resolved against the JSON file's directory.
	var b *builder.Builder
	if fromPath == "-" {
		b, err = fromjson.FromJSON(data)
	} else {
		b, err = fromjson.FromJSONFile(fromPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: parse error: %v\n", err)
		os.Exit(1)
	}
	rivBytes, err := b.Bytes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: build error: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if outputPath == "" {
		if _, err := os.Stdout.Write(rivBytes); err != nil {
			fmt.Fprintf(os.Stderr, "create: write stdout: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := os.WriteFile(outputPath, rivBytes, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "create: write %s: %v\n", outputPath, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "✓ wrote %s (%d bytes)\n", outputPath, len(rivBytes))
	}
}

// cmdValidateSchema validates a JSON scene file without building a .riv.
func cmdValidateSchema(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Cannot read file: %v\n", err)
		fmt.Println("Result: INVALID (1 error)")
		os.Exit(1)
	}

	all := fromjson.ValidateJSON(data)
	var warns, fatal []error
	for _, e := range all {
		if fromjson.IsWarning(e) {
			warns = append(warns, e)
		} else {
			fatal = append(fatal, e)
		}
	}
	for _, w := range warns {
		fmt.Printf("⚠ %v\n", w)
	}
	if len(fatal) == 0 {
		fmt.Println("✓ JSON schema valid")
		fmt.Println("Result: VALID")
		if len(warns) > 0 {
			fmt.Printf("  (%d warning(s))\n", len(warns))
		}
		return
	}
	for _, e := range fatal {
		fmt.Printf("✗ %v\n", e)
	}
	fmt.Printf("Result: INVALID (%d error(s))\n", len(fatal))
	os.Exit(1)
}

// ── inspect ───────────────────────────────────────────────────────────────────

func cmdInspect(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}

	f, err := rive.ReadBytes(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
		os.Exit(1)
	}

	totalProps := 0
	for _, o := range f.Objects {
		totalProps += len(o.Properties())
	}

	fmt.Printf("File:    %s (%d bytes)\n", path, len(raw))
	fmt.Printf("Major:   %d, Minor: %d, FileID: %d\n", f.MajorVersion, f.MinorVersion, f.FileID)
	fmt.Printf("Objects: %d, Properties: %d (total)\n\n", len(f.Objects), totalProps)

	// Build indent level based on parentId chain
	parentMap := make(map[int]int) // objIdx → parentIdx
	for i, o := range f.Objects {
		parentMap[i] = -1
		for _, p := range o.Properties() {
			if p.Key == 5 { // Component.parentId
				parentMap[i] = int(p.Value.(uint64))
			}
		}
	}

	// depth for display
	depths := make([]int, len(f.Objects))
	for i := range f.Objects {
		depths[i] = depthOf(i, parentMap, 0)
	}

	for i, o := range f.Objects {
		indent := strings.Repeat("  ", depths[i])
		typeName := typeKeyName(o.TypeKey())
		props := fmtProps(o.Properties())
		if props != "" {
			fmt.Printf("[%d]%s %s (typeKey=%d) %s\n", i, indent, typeName, o.TypeKey(), props)
		} else {
			fmt.Printf("[%d]%s %s (typeKey=%d)\n", i, indent, typeName, o.TypeKey())
		}
	}
}

func depthOf(idx int, parents map[int]int, recurse int) int {
	if recurse > 50 {
		return 0
	}
	p, ok := parents[idx]
	if !ok || p < 0 {
		return 0
	}
	return 1 + depthOf(p, parents, recurse+1)
}

func fmtProps(props []rive.Property) string {
	if len(props) == 0 {
		return ""
	}
	var parts []string
	for _, p := range props {
		switch p.Key {
		case 4, 55, 138: // name fields
			parts = append(parts, fmt.Sprintf("name=%q", p.Value.(string)))
		case 5: // parentId
			parts = append(parts, fmt.Sprintf("parentId=%d", p.Value.(uint64)))
		case 7: // width (Artboard)
			parts = append(parts, fmt.Sprintf("width=%.0f", p.Value.(float64)))
		case 8: // height (Artboard)
			parts = append(parts, fmt.Sprintf("height=%.0f", p.Value.(float64)))
		case 13: // x
			parts = append(parts, fmt.Sprintf("x=%.1f", p.Value.(float64)))
		case 14: // y
			parts = append(parts, fmt.Sprintf("y=%.1f", p.Value.(float64)))
		case 18: // opacity
			parts = append(parts, fmt.Sprintf("opacity=%.2f", p.Value.(float64)))
		case 20: // width (shape)
			parts = append(parts, fmt.Sprintf("w=%.0f", p.Value.(float64)))
		case 21: // height (shape)
			parts = append(parts, fmt.Sprintf("h=%.0f", p.Value.(float64)))
		case 37: // colorValue (SolidColor)
			parts = append(parts, fmt.Sprintf("color=0x%08X", uint32(p.Value.(uint64))))
		case 47: // thickness (Stroke)
			parts = append(parts, fmt.Sprintf("thickness=%.1f", p.Value.(float64)))
		case 51: // objectId (KeyedObject)
			parts = append(parts, fmt.Sprintf("objectId=%d", p.Value.(uint64)))
		case 53: // propertyKey (KeyedProperty)
			parts = append(parts, fmt.Sprintf("propKey=%d", p.Value.(uint64)))
		case 56: // fps
			parts = append(parts, fmt.Sprintf("fps=%d", p.Value.(uint64)))
		case 57: // duration
			parts = append(parts, fmt.Sprintf("dur=%d", p.Value.(uint64)))
		case 59: // loopValue
			loops := []string{"oneShot", "loop", "pingPong"}
			lv := p.Value.(uint64)
			if int(lv) < len(loops) {
				parts = append(parts, fmt.Sprintf("loop=%s", loops[lv]))
			}
		case 67: // frame
			parts = append(parts, fmt.Sprintf("frame=%d", p.Value.(uint64)))
		case 70: // keyframe value (float)
			parts = append(parts, fmt.Sprintf("val=%.4g", p.Value.(float64)))
		case 88: // keyframe color value
			// stored as uint64 color
			cv := p.Value.(uint64)
			parts = append(parts, fmt.Sprintf("color=0x%08X", uint32(cv)))
		case 149: // animationId
			parts = append(parts, fmt.Sprintf("animId=%d", p.Value.(uint64)))
		case 151: // stateToId
			parts = append(parts, fmt.Sprintf("stateTo=%d", p.Value.(uint64)))
		default:
			parts = append(parts, fmtUnknownProp(p))
		}
	}
	return strings.Join(parts, " ")
}

func fmtUnknownProp(p rive.Property) string {
	switch p.Type {
	case rive.PropertyTypeUint:
		return fmt.Sprintf("k%d=%d", p.Key, p.Value.(uint64))
	case rive.PropertyTypeFloat:
		return fmt.Sprintf("k%d=%.4g", p.Key, p.Value.(float64))
	case rive.PropertyTypeString:
		return fmt.Sprintf("k%d=%q", p.Key, p.Value.(string))
	case rive.PropertyTypeColor:
		cv := p.Value.(uint64)
		bits := uint32(cv)
		f := math.Float32frombits(bits)
		if f >= 0 && f <= 1 {
			return fmt.Sprintf("k%d=%.3f", p.Key, f)
		}
		return fmt.Sprintf("k%d=0x%08X", p.Key, bits)
	case rive.PropertyTypeBytes:
		b := p.Value.([]byte)
		if len(b) <= 8 {
			var parts []string
			for _, bb := range b {
				parts = append(parts, fmt.Sprintf("%02x", bb))
			}
			return fmt.Sprintf("k%d=[%s]", p.Key, strings.Join(parts, " "))
		}
		return fmt.Sprintf("k%d=[%d bytes]", p.Key, len(b))
	}
	return fmt.Sprintf("k%d=?", p.Key)
}

// fmtFloat32 formats a float stored in the color field (which is actually float32 bits).
// This is a helper for gradient position values (PropertyTypeColor used for float in some fields).
func fmtFloat32(bits uint32) string {
	f := math.Float32frombits(bits)
	return fmt.Sprintf("%.3f", f)
}

// ── validate ──────────────────────────────────────────────────────────────────

func cmdValidate(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("✗ Cannot read file: %v\n", err)
		fmt.Println("Result: INVALID (1 error)")
		return false
	}

	errors := 0
	report := func(ok bool, msg string) {
		if ok {
			fmt.Printf("✓ %s\n", msg)
		} else {
			fmt.Printf("✗ %s\n", msg)
			errors++
		}
	}

	// Check fingerprint manually
	if len(raw) >= 4 && string(raw[:4]) == "RIVE" {
		report(true, "Valid fingerprint (RIVE)")
	} else {
		got := string(raw[:min(4, len(raw))])
		report(false, fmt.Sprintf("Invalid fingerprint: expected RIVE, got %q", got))
	}

	if errors > 0 {
		fmt.Printf("Result: INVALID (%d error(s))\n", errors)
		return false
	}

	// Parse the file
	f, err := rive.ReadBytes(raw)
	if err != nil {
		fmt.Printf("✗ Parse error: %v\n", err)
		fmt.Println("Result: INVALID (1 error)")
		return false
	}

	report(f.MajorVersion == 7, fmt.Sprintf("Major version %d (expected 7)", f.MajorVersion))

	// Count properties
	totalProps := 0
	for _, o := range f.Objects {
		totalProps += len(o.Properties())
	}
	report(true, fmt.Sprintf("%d objects parsed", len(f.Objects)))
	report(true, fmt.Sprintf("%d total properties", totalProps))

	// Validate parentId references (artboard-relative resolution)
	orphans := 0
	badRefs := 0
	artboardOffset := -1
	for i, o := range f.Objects {
		if o.TypeKey() == 1 { // Artboard
			artboardOffset = i
		}
		for _, p := range o.Properties() {
			if p.Key == 5 { // parentId
				rel := int(p.Value.(uint64))
				if artboardOffset < 0 {
					badRefs++
					fmt.Printf("  ✗ Object[%d] has parentId=%d but no artboard seen yet\n", i, rel)
					continue
				}
				global := artboardOffset + rel
				if global < 0 || global >= len(f.Objects) {
					badRefs++
					fmt.Printf("  ✗ Object[%d] parentId=%d → global=%d out of range\n", i, rel, global)
				} else if global == i {
					orphans++
					fmt.Printf("  ✗ Object[%d] parentId points to itself\n", i)
				}
			}
		}
	}
	report(badRefs == 0, fmt.Sprintf("All parentId references valid (%d objects with parentId)", countParentIds(f.Objects)))
	report(orphans == 0, "No self-referencing objects")

	// Validate shapes have path and paint children (per-artboard)
	shapeChecks := validateShapeStructure(f.Objects)
	report(shapeChecks == 0, "Shape structure valid (paths + paints present)")

	// Validate KeyedObject.objectId references
	badObjIds := 0
	for i, o := range f.Objects {
		if o.TypeKey() != 25 { // KeyedObject
			continue
		}
		for _, p := range o.Properties() {
			if p.Key == 51 { // objectId
				oid := int(p.Value.(uint64))
				if oid < 0 || oid >= len(f.Objects) {
					badObjIds++
					fmt.Printf("  ✗ Object[%d] KeyedObject.objectId=%d out of range\n", i, oid)
				}
			}
		}
	}
	report(badObjIds == 0, "All KeyedObject.objectId references valid")

	if errors > 0 {
		fmt.Printf("Result: INVALID (%d error(s))\n", errors)
		return false
	}
	fmt.Println("Result: VALID")
	return true
}

// validateShapeStructure returns the count of artboards with shape-but-no-path or shape-but-no-paint.
func validateShapeStructure(objects []rive.Object) int {
	// Find artboard ranges
	type abRange struct{ start, end int }
	var ranges []abRange
	for i, o := range objects {
		if o.TypeKey() == 1 {
			ranges = append(ranges, abRange{start: i})
		}
	}
	for i := range ranges {
		if i+1 < len(ranges) {
			ranges[i].end = ranges[i+1].start
		} else {
			ranges[i].end = len(objects)
		}
	}

	pathKeys := map[uint32]bool{7: true, 4: true, 8: true, 16: true} // Rect, Ellipse, Triangle, PointsPath
	problems := 0
	for _, ar := range ranges {
		slice := objects[ar.start:ar.end]
		nShapes := countObjects(slice, 3)
		if nShapes == 0 {
			continue
		}
		nPaths := 0
		for pk := range pathKeys {
			nPaths += countObjects(slice, pk)
		}
		nPaints := countObjects(slice, 20) + countObjects(slice, 24) // Fill + Stroke
		if nPaths == 0 {
			fmt.Printf("  ✗ Artboard[%d]: %d shape(s) but no path objects\n", ar.start, nShapes)
			problems++
		}
		if nPaints == 0 {
			fmt.Printf("  ✗ Artboard[%d]: %d shape(s) but no paint objects\n", ar.start, nShapes)
			problems++
		}
	}
	return problems
}

func countObjects(objects []rive.Object, typeKey uint32) int {
	n := 0
	for _, o := range objects {
		if o.TypeKey() == typeKey {
			n++
		}
	}
	return n
}

func countParentIds(objects []rive.Object) int {
	n := 0
	for _, o := range objects {
		for _, p := range o.Properties() {
			if p.Key == 5 {
				n++
			}
		}
	}
	return n
}

// ── generate ──────────────────────────────────────────────────────────────────

func cmdGenerate() {
	// Re-exec the examples program by running the same package.
	// Since rivtool is a separate binary, we delegate by shell.
	fmt.Println("Run: go run ./cmd/examples/ to regenerate examples.")
}

// ── type name table ───────────────────────────────────────────────────────────

func typeKeyName(k uint32) string {
	names := map[uint32]string{
		1:   "Artboard",
		2:   "Node",
		3:   "Shape",
		4:   "Ellipse",
		5:   "StraightVertex",
		6:   "CubicDetachedVertex",
		7:   "Rectangle",
		8:   "Triangle",
		16:  "PointsPath",
		17:  "RadialGradient",
		18:  "SolidColor",
		19:  "GradientStop",
		20:  "Fill",
		21:  "ShapePaint",
		22:  "LinearGradient",
		23:  "Backboard",
		24:  "Stroke",
		25:  "KeyedObject",
		26:  "KeyedProperty",
		30:  "KeyFrameDouble",
		31:  "LinearAnimation",
		37:  "KeyFrameColor",
		38:  "WorldTransformComponent",
		42:  "ClippingShape",
		47:  "TrimPath",
		48:  "DrawTarget",
		49:  "DrawRules",
		50:  "KeyFrameId",
		51:  "Polygon",
		52:  "Star",
		53:  "StateMachine",
		55:  "StateMachineInput",
		56:  "StateMachineNumber",
		57:  "StateMachineLayer",
		58:  "StateMachineTrigger",
		59:  "StateMachineBool",
		61:  "AnimationState",
		62:  "AnyState",
		63:  "EntryState",
		64:  "ExitState",
		65:  "StateTransition",
		67:  "TransitionInputCondition",
		68:  "TransitionTriggerCondition",
		70:  "TransitionNumberCondition",
		71:  "TransitionBoolCondition",
		84:  "KeyFrameBool",
		91:  "WorldTransformComponent",
		100: "Image",
		109: "Mesh",
		170: "InterpolatingKeyFrame",
		450: "KeyFrameUint",
	}
	if n, ok := names[k]; ok {
		return n
	}
	return "Unknown"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Silence unused import warning — binary is used in fmtFloat32 indirectly.
var _ = binary.LittleEndian
