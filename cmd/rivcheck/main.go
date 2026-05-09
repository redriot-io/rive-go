// rivcheck: uses rive.ReadBytes to correctly parse and inspect .riv files
package main

import (
	"fmt"
	"os"

	"github.com/redriot-io/rive-go/rive"
)

var typeNames = map[uint32]string{
	1: "Artboard", 3: "Shape", 4: "Ellipse", 7: "Rectangle",
	17: "RadialGradient", 18: "SolidColor", 19: "GradientStop",
	20: "Fill", 22: "LinearGradient", 23: "Backboard",
	25: "KeyedObject", 26: "KeyedProperty", 28: "CubicInterp",
	30: "KeyFrameDouble", 31: "LinearAnimation", 37: "KeyFrameColor",
}

func typeName(k uint32) string {
	if n, ok := typeNames[k]; ok {
		return n
	}
	return fmt.Sprintf("Unk(%d)", k)
}

func getPropUint(obj rive.Object, key uint32) (uint64, bool) {
	for _, p := range obj.Properties() {
		if p.Key == key {
			if v, ok := p.Value.(uint64); ok {
				return v, true
			}
		}
	}
	return 0, false
}

func getPropFloat(obj rive.Object, key uint32) (float64, bool) {
	for _, p := range obj.Properties() {
		if p.Key == key {
			if v, ok := p.Value.(float64); ok {
				return v, true
			}
		}
	}
	return 0, false
}

func getPropString(obj rive.Object, key uint32) (string, bool) {
	for _, p := range obj.Properties() {
		if p.Key == key {
			if v, ok := p.Value.(string); ok {
				return v, true
			}
		}
	}
	return "", false
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rivcheck <file.riv> [file2.riv ...]")
		os.Exit(1)
	}

	for _, path := range os.Args[1:] {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
			continue
		}

		f, err := rive.ReadBytes(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
			continue
		}

		fmt.Printf("\n=== %s (%d bytes) ===\n", path, len(data))
		fmt.Printf("Version: %d.%d, FileID: %d, Objects: %d\n",
			f.MajorVersion, f.MinorVersion, f.FileID, len(f.Objects))

		// Find artboard position
		artboardIdx := -1
		for i, o := range f.Objects {
			if o.TypeKey() == 1 { // Artboard
				artboardIdx = i
				break
			}
		}
		if artboardIdx < 0 {
			fmt.Println("  ERROR: No Artboard found!")
			continue
		}
		fmt.Printf("Artboard at global[%d] (artboardOffset=%d)\n\n", artboardIdx, artboardIdx)

		// Print table
		fmt.Printf("%-4s %-4s %-6s %-20s %-8s %-8s %-8s %-8s extras\n",
			"Glob", "ARel", "Type", "TypeName", "parentId", "objectId", "propKey", "name/val")
		fmt.Println("-----------------------------------------------------------------------------------------------------------")

		for i, obj := range f.Objects {
			aRel := ""
			arRelInt := i - artboardIdx
			if i >= artboardIdx {
				aRel = fmt.Sprintf("%d", arRelInt)
			}

			// parentId = key 7
			parentId := ""
			if v, ok := getPropUint(obj, 5); ok {
				parentId = fmt.Sprintf("%d", v)
			}
			// objectId = key 51
			objectId := ""
			if v, ok := getPropUint(obj, 51); ok {
				objectId = fmt.Sprintf("%d", v)
			}
			// propKey = key 53
			propKey := ""
			if v, ok := getPropUint(obj, 53); ok {
				propKey = fmt.Sprintf("%d", v)
			}
			// name = key 4
			name := ""
			if v, ok := getPropString(obj, 4); ok && v != "" {
				name = fmt.Sprintf("name=%q", v)
			}
			// x = key 13, y = key 14
			extras := name
			if x, ok := getPropFloat(obj, 13); ok {
				extras += fmt.Sprintf(" x=%.1f", x)
			}
			if y, ok := getPropFloat(obj, 14); ok {
				extras += fmt.Sprintf(" y=%.1f", y)
			}
			// width/height for rect/ellipse
			if w, ok := getPropFloat(obj, 20); ok {
				extras += fmt.Sprintf(" w=%.0f", w)
			}
			if h, ok := getPropFloat(obj, 21); ok {
				extras += fmt.Sprintf(" h=%.0f", h)
			}

			fmt.Printf("%-4d %-4s %-6d %-20s %-8s %-8s %-8s %s\n",
				i, aRel, obj.TypeKey(), typeName(obj.TypeKey()),
				parentId, objectId, propKey, extras)
		}

		// Verify animation targets
		fmt.Println("\n--- Animation Target Verification ---")
		for i, obj := range f.Objects {
			if obj.TypeKey() == 25 { // KeyedObject
				if objID, ok := getPropUint(obj, 51); ok {
					targetGlobal := artboardIdx + int(objID)
					fmt.Printf("  [%d] KeyedObject objectId=%d → global[%d]", i, objID, targetGlobal)
					if targetGlobal < len(f.Objects) {
						target := f.Objects[targetGlobal]
						fmt.Printf(" = %s", typeName(target.TypeKey()))
						if n, ok := getPropString(target, 4); ok {
							fmt.Printf(" %q", n)
						}
					} else {
						fmt.Printf(" OUT-OF-BOUNDS (max=%d)", len(f.Objects)-1)
					}
					fmt.Println()
				} else {
					fmt.Printf("  [%d] KeyedObject: objectId=0 (default) → global[%d] = Artboard\n",
						i, artboardIdx)
				}
			}
		}
	}
}
