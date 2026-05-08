// rivdiff: annotated decoder and structural diff for .riv files.
//
// Usage:
//
//	rivdiff <file.riv>               # decode and annotate a single file
//	rivdiff <file-a.riv> <file-b.riv> # decode both and show structural diff
package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/redriot-io/rive-go/rive"
)

// ── Property key → name (common subset) ──────────────────────────────────────

var propNames = map[uint32]string{
	4:   "Component.Name",
	5:   "Node.ParentId",
	7:   "LayoutComponent.Width",
	8:   "LayoutComponent.Height",
	11:  "Artboard.OriginX",
	12:  "Artboard.OriginY",
	13:  "Node.X",
	14:  "Node.Y",
	15:  "Node.Rotation",
	16:  "Node.ScaleX",
	17:  "Node.ScaleY",
	18:  "Node.Opacity",
	20:  "ParametricPath.Width",
	21:  "ParametricPath.Height",
	22:  "ParametricPath.OriginX",
	23:  "ParametricPath.OriginY",
	31:  "Rectangle.CornerRadius",
	37:  "SolidColor.ColorValue",
	41:  "ShapePaint.IsVisible",
	51:  "KeyedObject.ObjectId",
	53:  "KeyedProperty.PropertyKey",
	55:  "Animation.Name",
	56:  "LinearAnimation.Fps",
	57:  "LinearAnimation.Duration",
	58:  "LinearAnimation.Speed",
	59:  "LinearAnimation.LoopValue",
	60:  "LinearAnimation.WorkStart",
	61:  "LinearAnimation.WorkEnd",
	62:  "LinearAnimation.EnableWorkArea",
	67:  "KeyFrame.Frame",
	68:  "InterpolatingKeyFrame.InterpolationType",
	69:  "InterpolatingKeyFrame.InterpolatorId",
	70:  "KeyFrameDouble.Value",
	72:  "KeyFrameColor.Value",
	128: "Stroke.Thickness",
	138: "StateMachine.Name",
	139: "StateMachine.MachineOrder",
	164: "Rectangle.LinkCornerRadius",
	196: "LayoutComponent.Clip",
	197: "NestedArtboard.ArtboardId",
	198: "NestedAnimation.AnimationId",
	236: "Artboard.DefaultStateMachineId",
	292: "AdvanceableState.Speed",
	376: "LinearAnimation.Quantize",
	494: "LayoutComponent.StyleId",
	583: "Artboard.ViewModelId",
	706: "LayoutComponent.FractionalWidth",
	707: "LayoutComponent.FractionalHeight",
	747: "ShapePaint.BlendModeValue",
	951: "Artboard.IsStateful",
}

var typeKeyNames = map[uint32]string{
	1:   "Artboard",
	3:   "Shape",
	4:   "Ellipse",
	7:   "Rectangle",
	18:  "SolidColor",
	19:  "GradientStop",
	20:  "Fill",
	21:  "Stroke",
	22:  "LinearGradient",
	23:  "Backboard",
	25:  "KeyedObject",
	26:  "KeyedProperty",
	27:  "Animation",
	28:  "CubicEaseInterpolator",
	30:  "KeyFrameDouble",
	31:  "LinearAnimation",
	37:  "KeyFrameColor",
	42:  "ClippingShape",
	51:  "Polygon",
	52:  "Star",
	56:  "Path",
	57:  "PointsPath",
	61:  "AnimationState",
	62:  "AnyState",
	63:  "EntryState",
	64:  "ExitState",
	65:  "TransitionCondition",
	67:  "StateMachineLayer",
	72:  "StateMachine",
	74:  "BlendAnimation",
	75:  "BlendAnimation1D",
	77:  "BlendAnimationDirect",
	88:  "StateMachineTransition",
	92:  "NestedArtboard",
	93:  "NestedAnimation",
	95:  "NestedStateMachine",
	97:  "NestedLinearAnimation",
	100: "Image",
	109: "Mesh",
	128: "Event",
	138: "CubicValueInterpolator",
	140: "Guide",
	143: "AnimationFolder",
	163: "CubicInterpolatorComponent",
	409: "LayoutComponent",
	420: "LayoutComponentStyle",
}

var propTypeNames = [5]string{"uint", "string", "float32", "color", "bytes"}

// ── Low-level byte reader ─────────────────────────────────────────────────────

type byteReader struct {
	data []byte
	pos  int
}

func newByteReader(data []byte) *byteReader { return &byteReader{data: data} }

func (r *byteReader) eof() bool { return r.pos >= len(r.data) }

func (r *byteReader) remaining() int { return len(r.data) - r.pos }

func (r *byteReader) readByte() (byte, error) {
	if r.eof() {
		return 0, fmt.Errorf("EOF at offset 0x%X", r.pos)
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func (r *byteReader) readVarUint() (uint64, error) {
	var v uint64
	shift := 0
	for {
		b, err := r.readByte()
		if err != nil {
			return 0, err
		}
		v |= uint64(b&0x7F) << uint(shift)
		shift += 7
		if b&0x80 == 0 {
			break
		}
		if shift > 63 {
			return 0, fmt.Errorf("varuint overflow at offset 0x%X", r.pos)
		}
	}
	return v, nil
}

func (r *byteReader) readUint32LE() (uint32, error) {
	if r.remaining() < 4 {
		return 0, fmt.Errorf("need 4 bytes for uint32 at offset 0x%X, have %d", r.pos, r.remaining())
	}
	v := binary.LittleEndian.Uint32(r.data[r.pos : r.pos+4])
	r.pos += 4
	return v, nil
}

func (r *byteReader) readFloat32LE() (float32, error) {
	v, err := r.readUint32LE()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(v), nil
}

func (r *byteReader) readString() (string, error) {
	n, err := r.readVarUint()
	if err != nil {
		return "", err
	}
	if int(n) > r.remaining() {
		return "", fmt.Errorf("string length %d exceeds remaining %d at offset 0x%X", n, r.remaining(), r.pos)
	}
	s := string(r.data[r.pos : r.pos+int(n)])
	r.pos += int(n)
	return s, nil
}

// ── Decoded types ─────────────────────────────────────────────────────────────

type decodedProp struct {
	key      uint32
	ptype    rive.PropertyType
	uval     uint64
	sval     string
	fval     float32
	cval     uint32
	bval     []byte
	startOff int
	endOff   int
}

type decodedObject struct {
	index    int
	typeKey  uint32
	props    []decodedProp
	startOff int
	endOff   int
}

type decodedToCEntry struct {
	key   uint32
	ptype rive.PropertyType
}

type decodedFile struct {
	path       string
	raw        []byte
	major      uint32
	minor      uint32
	fileID     uint64
	toc        []decodedToCEntry
	tocKeyEnd  int
	tocBitsEnd int
	objects    []decodedObject
	err        error
}

// ── Decoder ───────────────────────────────────────────────────────────────────

func decodeFile(path string, data []byte) *decodedFile {
	f := &decodedFile{path: path, raw: data}
	r := newByteReader(data)

	if r.remaining() < 4 {
		f.err = fmt.Errorf("file too short for fingerprint (%d bytes)", len(data))
		return f
	}
	if string(data[0:4]) != "RIVE" {
		f.err = fmt.Errorf("bad fingerprint %q (want \"RIVE\")", string(data[0:4]))
		return f
	}
	r.pos = 4

	maj, err := r.readVarUint()
	if err != nil {
		f.err = fmt.Errorf("major version: %w", err)
		return f
	}
	f.major = uint32(maj)

	min, err := r.readVarUint()
	if err != nil {
		f.err = fmt.Errorf("minor version: %w", err)
		return f
	}
	f.minor = uint32(min)

	fileID, err := r.readVarUint()
	if err != nil {
		f.err = fmt.Errorf("fileID: %w", err)
		return f
	}
	f.fileID = fileID

	var tocKeys []uint32
	for {
		k, err := r.readVarUint()
		if err != nil {
			f.err = fmt.Errorf("ToC key: %w", err)
			return f
		}
		if k == 0 {
			break
		}
		tocKeys = append(tocKeys, uint32(k))
	}
	f.tocKeyEnd = r.pos

	tocTypeMap := make(map[uint32]rive.PropertyType, len(tocKeys))
	tocEntries := make([]decodedToCEntry, len(tocKeys))
	{
		var currentWord uint32
		currentBit := 8
		for i, k := range tocKeys {
			if currentBit == 8 {
				w, err := r.readUint32LE()
				if err != nil {
					f.err = fmt.Errorf("ToC type bits: %w", err)
					return f
				}
				currentWord = w
				currentBit = 0
			}
			pt := rive.PropertyType((currentWord >> uint(currentBit)) & 3)
			tocTypeMap[k] = pt
			tocEntries[i] = decodedToCEntry{key: k, ptype: pt}
			currentBit += 2
		}
	}
	f.toc = tocEntries
	f.tocBitsEnd = r.pos

	objIdx := 0
	for !r.eof() {
		objStart := r.pos
		typeKey, err := r.readVarUint()
		if err != nil {
			f.err = fmt.Errorf("object %d typeKey: %w", objIdx, err)
			return f
		}
		if typeKey == 0 {
			break
		}

		obj := decodedObject{index: objIdx, typeKey: uint32(typeKey), startOff: objStart}
		propIdx := 0
		for {
			propStart := r.pos
			propKey, err := r.readVarUint()
			if err != nil {
				f.err = fmt.Errorf("object %d prop %d key: %w", objIdx, propIdx, err)
				return f
			}
			if propKey == 0 {
				break
			}

			pt, ok := tocTypeMap[uint32(propKey)]
			if !ok {
				pt, ok = rive.LookupGlobalPropType(uint32(propKey))
				if !ok {
					f.err = fmt.Errorf("object %d prop key %d: unknown type", objIdx, propKey)
					return f
				}
			}

			dp := decodedProp{key: uint32(propKey), ptype: pt, startOff: propStart}
			switch pt {
			case rive.PropertyTypeUint:
				v, err := r.readVarUint()
				if err != nil {
					f.err = fmt.Errorf("object %d prop %d (uint): %w", objIdx, propIdx, err)
					return f
				}
				dp.uval = v
			case rive.PropertyTypeString:
				s, err := r.readString()
				if err != nil {
					f.err = fmt.Errorf("object %d prop %d (string): %w", objIdx, propIdx, err)
					return f
				}
				dp.sval = s
			case rive.PropertyTypeFloat:
				v, err := r.readFloat32LE()
				if err != nil {
					f.err = fmt.Errorf("object %d prop %d (float): %w", objIdx, propIdx, err)
					return f
				}
				dp.fval = v
			case rive.PropertyTypeColor:
				v, err := r.readUint32LE()
				if err != nil {
					f.err = fmt.Errorf("object %d prop %d (color): %w", objIdx, propIdx, err)
					return f
				}
				dp.cval = v
			case rive.PropertyTypeBytes:
				n, err := r.readVarUint()
				if err != nil {
					f.err = fmt.Errorf("object %d prop %d (bytes len): %w", objIdx, propIdx, err)
					return f
				}
				if int(n) > r.remaining() {
					f.err = fmt.Errorf("object %d prop %d bytes: length %d > remaining %d", objIdx, propIdx, n, r.remaining())
					return f
				}
				dp.bval = make([]byte, n)
				copy(dp.bval, r.data[r.pos:r.pos+int(n)])
				r.pos += int(n)
			}
			dp.endOff = r.pos
			obj.props = append(obj.props, dp)
			propIdx++
		}
		obj.endOff = r.pos
		f.objects = append(f.objects, obj)
		objIdx++
	}

	return f
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func propTypeName(pt rive.PropertyType) string {
	if int(pt) < len(propTypeNames) {
		return propTypeNames[pt]
	}
	return fmt.Sprintf("type(%d)", pt)
}

func propKeyName(key uint32) string {
	if n, ok := propNames[key]; ok {
		return n
	}
	return fmt.Sprintf("key#%d", key)
}

func typeKeyName(tk uint32) string {
	if n, ok := typeKeyNames[tk]; ok {
		return n
	}
	return fmt.Sprintf("TypeKey#%d", tk)
}

func formatPropValue(dp decodedProp) string {
	switch dp.ptype {
	case rive.PropertyTypeUint:
		return fmt.Sprintf("%d", dp.uval)
	case rive.PropertyTypeString:
		return fmt.Sprintf("%q", dp.sval)
	case rive.PropertyTypeFloat:
		return fmt.Sprintf("%g", dp.fval)
	case rive.PropertyTypeColor:
		return fmt.Sprintf("0x%08X (A=%02X R=%02X G=%02X B=%02X)",
			dp.cval,
			(dp.cval>>24)&0xFF,
			(dp.cval>>16)&0xFF,
			(dp.cval>>8)&0xFF,
			dp.cval&0xFF,
		)
	case rive.PropertyTypeBytes:
		if len(dp.bval) > 16 {
			return fmt.Sprintf("[%d bytes: %s...]", len(dp.bval), hex.EncodeToString(dp.bval[:8]))
		}
		return fmt.Sprintf("[%d bytes: %s]", len(dp.bval), hex.EncodeToString(dp.bval))
	}
	return "?"
}

func hexChunk(data []byte, start, end int) string {
	if end > len(data) {
		end = len(data)
	}
	if start >= end {
		return ""
	}
	chunk := data[start:end]
	if len(chunk) > 12 {
		return hex.EncodeToString(chunk[:12]) + "..."
	}
	return hex.EncodeToString(chunk)
}

func printFile(f *decodedFile) {
	fmt.Printf("╔══ %s (%d bytes) ══\n", f.path, len(f.raw))

	if f.err != nil {
		fmt.Printf("  DECODE ERROR: %v\n", f.err)
		limit := 64
		if len(f.raw) < limit {
			limit = len(f.raw)
		}
		fmt.Printf("  Raw hex (first %d bytes):\n  %s\n", limit, hex.EncodeToString(f.raw[:limit]))
		return
	}

	fmt.Printf("  HEADER  major=%d  minor=%d  fileID=%d\n", f.major, f.minor, f.fileID)
	fmt.Printf("  TOC     %d keys  (keys 0x04–0x%02X, type-words 0x%02X–0x%02X)\n",
		len(f.toc), f.tocKeyEnd-1, f.tocKeyEnd, f.tocBitsEnd-1)

	for i, e := range f.toc {
		fmt.Printf("    [%2d] key=%-5d  %-8s  %s\n",
			i, e.key, propTypeName(e.ptype), propKeyName(e.key))
	}

	fmt.Printf("  OBJECTS %d  (stream starts 0x%02X)\n", len(f.objects), f.tocBitsEnd)
	for _, obj := range f.objects {
		fmt.Printf("  [%d] %s (typeKey=%d)  @0x%X–0x%X  (%d bytes)\n",
			obj.index, typeKeyName(obj.typeKey), obj.typeKey,
			obj.startOff, obj.endOff, obj.endOff-obj.startOff)
		for _, p := range obj.props {
			rawHex := hexChunk(f.raw, p.startOff, p.endOff)
			fmt.Printf("       %-42s = %-36s  [%s]\n",
				propKeyName(p.key), formatPropValue(p), rawHex)
		}
	}
	fmt.Println()
}

// ── Structural diff ───────────────────────────────────────────────────────────

func structureLines(f *decodedFile) []string {
	var lines []string
	lines = append(lines, fmt.Sprintf("header: major=%d minor=%d fileID=%d", f.major, f.minor, f.fileID))
	lines = append(lines, fmt.Sprintf("toc(%d):", len(f.toc)))
	for _, e := range f.toc {
		lines = append(lines, fmt.Sprintf("  key=%d/%s:%s", e.key, propKeyName(e.key), propTypeName(e.ptype)))
	}
	lines = append(lines, fmt.Sprintf("objects(%d):", len(f.objects)))
	for _, obj := range f.objects {
		lines = append(lines, fmt.Sprintf("  [%d]%s", obj.index, typeKeyName(obj.typeKey)))
		for _, p := range obj.props {
			lines = append(lines, fmt.Sprintf("    %s=%s", propKeyName(p.key), formatPropValue(p)))
		}
	}
	return lines
}

func diffFiles(fa, fb *decodedFile) {
	fmt.Printf("╔══ STRUCTURAL DIFF: %s ↔ %s ══\n", fa.path, fb.path)

	if fa.err != nil {
		fmt.Printf("  A has decode error: %v\n", fa.err)
	}
	if fb.err != nil {
		fmt.Printf("  B has decode error: %v\n", fb.err)
	}
	if fa.err != nil || fb.err != nil {
		return
	}

	linesA := structureLines(fa)
	linesB := structureLines(fb)

	maxLen := len(linesA)
	if len(linesB) > maxLen {
		maxLen = len(linesB)
	}

	hasDiff := false
	for i := 0; i < maxLen; i++ {
		var a, b string
		if i < len(linesA) {
			a = linesA[i]
		} else {
			a = "<missing>"
		}
		if i < len(linesB) {
			b = linesB[i]
		} else {
			b = "<missing>"
		}
		if a != b {
			hasDiff = true
			fmt.Printf("  - A: %s\n", a)
			fmt.Printf("  + B: %s\n", b)
		}
	}

	if !hasDiff {
		fmt.Println("  (structures are identical)")
	}
	fmt.Println()
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rivdiff <file.riv> [file-b.riv]")
		fmt.Fprintln(os.Stderr, "  Single file: annotated hex decode")
		fmt.Fprintln(os.Stderr, "  Two files:   annotated decode of each + structural diff")
		os.Exit(1)
	}

	files := make([]*decodedFile, len(args))
	for i, path := range args {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
			os.Exit(1)
		}
		files[i] = decodeFile(path, data)
	}

	sep := strings.Repeat("─", 80)
	for _, f := range files {
		fmt.Println(sep)
		printFile(f)
	}

	if len(args) == 2 {
		fmt.Println(sep)
		diffFiles(files[0], files[1])
	}
}
