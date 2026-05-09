// rivtool inspects, validates, and creates Rive (.riv) files.
//
// Usage:
//
//	rivtool inspect <file.riv>                — dump object tree with properties
//	rivtool validate <file.riv>               — check well-formedness of a .riv
//	rivtool validate --schema <scene.json>    — validate a JSON scene file
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
	case "generate":
		cmdGenerate()
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
  create   --from <scene.json>           build .riv from JSON scene, write to stdout
  create   --from <scene.json> --output <out.riv>
  create   --from -                      read JSON from stdin
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

	// Parse and build
	b, err := fromjson.FromJSON(data)
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

	errs := fromjson.ValidateJSON(data)
	if len(errs) == 0 {
		fmt.Println("✓ JSON schema valid")
		fmt.Println("Result: VALID")
		return
	}

	for _, e := range errs {
		fmt.Printf("✗ %v\n", e)
	}
	fmt.Printf("Result: INVALID (%d error(s))\n", len(errs))
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
