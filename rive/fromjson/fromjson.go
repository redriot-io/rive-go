// Package fromjson converts a JSON scene description into a rive-go Builder.
//
// JSON format version 1 supports artboards with rectangle/ellipse shapes,
// solid-color, linear-gradient, and radial-gradient fills, stroke,
// float/color animations, and state machines with bool/number/trigger inputs.
package fromjson

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/redriot-io/rive-go/rive/builder"
)

// ── JSON schema types ─────────────────────────────────────────────────────────

// Scene is the top-level JSON document.
type Scene struct {
	Version  int      `json:"version"`
	Artboard Artboard `json:"artboard"`
}

// FontDef describes a font to embed in the scene.
type FontDef struct {
	Name string `json:"name"` // logical name used in style references
	File string `json:"file"` // TTF/OTF path relative to the JSON file (FromJSONFile only)
}

// TextStyleDef describes the style properties of a text child.
type TextStyleDef struct {
	Font          string  `json:"font"`                    // matches a FontDef.Name
	FontSize      float64 `json:"fontSize"`
	Fill          string  `json:"fill,omitempty"`          // hex color e.g. "#FF0000"
	LineHeight    float64 `json:"lineHeight,omitempty"`    // 0 = auto
	LetterSpacing float64 `json:"letterSpacing,omitempty"`
}

// NamedStyleDef is one entry in the multi-run `"styles"` array.
// The Name field is used by RunDef.Style to reference this style.
type NamedStyleDef struct {
	Name          string  `json:"name"`
	Font          string  `json:"font"`
	FontSize      float64 `json:"fontSize"`
	Fill          string  `json:"fill,omitempty"`
	LineHeight    float64 `json:"lineHeight,omitempty"`
	LetterSpacing float64 `json:"letterSpacing,omitempty"`
}

// RunDef is one text span in the multi-run `"runs"` array.
// Style must match a NamedStyleDef.Name declared in the same text object's `"styles"`.
type RunDef struct {
	Text  string `json:"text"`
	Style string `json:"style"`
}

// Artboard describes the canvas and its children.
type Artboard struct {
	Name          string         `json:"name"`
	Width         float64        `json:"width"`
	Height        float64        `json:"height"`
	Fonts         []FontDef      `json:"fonts,omitempty"`
	Children      []Child        `json:"children,omitempty"`
	Animations    []AnimationDef `json:"animations,omitempty"`
	StateMachines []SMDef        `json:"state_machines,omitempty"`
}

// Child is a shape node (rectangle or ellipse).
type Child struct {
	Type     string          `json:"type"` // "rectangle" | "ellipse"
	Name     string          `json:"name"`
	X        float64         `json:"x"`
	Y        float64         `json:"y"`
	Width    float64         `json:"width"`
	Height   float64         `json:"height"`
	Rotation     float64         `json:"rotation,omitempty"` // degrees
	ScaleX       float64         `json:"scaleX,omitempty"`
	ScaleY       float64         `json:"scaleY,omitempty"`
	Opacity      float64         `json:"opacity,omitempty"`
	CornerRadius float64         `json:"corner_radius,omitempty"` // rectangles only
	Fill      json.RawMessage `json:"fill,omitempty"`
	Stroke    *StrokeDef      `json:"stroke,omitempty"`
	DrawRules []DrawRuleDef   `json:"draw_rules,omitempty"`
	// path-specific fields
	Vertices []VertexDef    `json:"vertices,omitempty"`
	Closed   bool           `json:"closed,omitempty"`
	Clip     string         `json:"clip,omitempty"` // name of a path child to use as clip source
	// text-specific fields — single-run format (mutually exclusive with Styles/Runs)
	Text  string        `json:"text,omitempty"`  // for type="text"
	Style *TextStyleDef `json:"style,omitempty"` // for type="text"
	Align    string `json:"align,omitempty"`    // "left"|"center"|"right" for type="text"
	Overflow string `json:"overflow,omitempty"` // "visible"|"hidden"|"clipped"|"ellipsis"|"fit"
	Sizing   string `json:"sizing,omitempty"`   // "auto_width"|"auto_height"|"fixed"
	// Multi-run format: use Styles + Runs instead of Style + Text.
	Styles []NamedStyleDef `json:"styles,omitempty"`
	Runs   []RunDef        `json:"runs,omitempty"`
}

// DrawRuleDef describes one draw-order constraint on a shape.
type DrawRuleDef struct {
	Type   string `json:"type"`   // "above" | "below"
	Target string `json:"target"` // name of the reference shape
}

// StrokeDef describes a stroke paint.
type StrokeDef struct {
	Color string  `json:"color"`
	Width float64 `json:"width"`
}

// fillObj is the object form of a fill.
type fillObj struct {
	Type    string         `json:"type"` // "solid" | "linear_gradient" | "radial_gradient"
	Color   string         `json:"color,omitempty"`
	Opacity float64        `json:"opacity,omitempty"`
	// linear_gradient fields
	Start   [2]float64     `json:"start,omitempty"`
	End     [2]float64     `json:"end,omitempty"`
	// radial_gradient fields (shape-local pixel coords)
	Center  [2]float64     `json:"center,omitempty"`
	Radius  float64        `json:"radius,omitempty"`
	Stops   []gradientStop `json:"stops,omitempty"`
}

type gradientStop struct {
	Position float64 `json:"position"`
	Color    string  `json:"color"`
}

// VertexDef describes one vertex in a custom path.
type VertexDef struct {
	X      float64   `json:"x"`
	Y      float64   `json:"y"`
	Radius float64   `json:"radius,omitempty"` // straight vertex corner radius
	In     []float64 `json:"in,omitempty"`     // [inX, inY] cubic in-handle (absolute coords)
	Out    []float64 `json:"out,omitempty"`    // [outX, outY] cubic out-handle (absolute coords)
}

// AnimationDef describes a linear animation.
type AnimationDef struct {
	Name            string            `json:"name"`
	Duration        float64           `json:"duration"` // seconds
	FPS             float64           `json:"fps,omitempty"`
	Loop            string            `json:"loop,omitempty"` // "oneshot"|"loop"|"pingpong"
	Speed           float64           `json:"speed,omitempty"`
	Tracks          []TrackDef        `json:"tracks"`
	DrawOrderTracks []DrawOrderTrackDef `json:"draw_order_tracks,omitempty"`
}

// DrawOrderTrackDef keys a shape's draw order across the animation timeline.
// Each keyframe switches which shape the source draws above or below (hold semantics).
type DrawOrderTrackDef struct {
	Shape     string            `json:"shape"`     // name of the source shape
	Keyframes []DrawOrderKFDef  `json:"keyframes"`
}

// DrawOrderKFDef is one draw-order keyframe.
type DrawOrderKFDef struct {
	Time      float64 `json:"time"`                // seconds
	Target    string  `json:"target,omitempty"`    // target shape name; omit/empty = reset to default
	Placement string  `json:"placement,omitempty"` // "above" (default) | "below"
}

// TrackDef animates one property of one shape.
type TrackDef struct {
	Target    string        `json:"target"` // dot-path, e.g. "box.rotation"
	Keyframes []KeyframeDef `json:"keyframes"`
}

// KeyframeDef is one keyframe in a track.
type KeyframeDef struct {
	Time   float64         `json:"time"`   // seconds
	Value  json.RawMessage `json:"value"`  // number or "#RRGGBB"
	Easing json.RawMessage `json:"easing,omitempty"` // "linear"|"ease-out"|cubic object
}

// SMDef describes a state machine.
type SMDef struct {
	Name      string          `json:"name"`
	Inputs    []SMInput       `json:"inputs,omitempty"`
	Layers    []SMLayer       `json:"layers"`
	Listeners []SMListenerDef `json:"listeners,omitempty"`
}

// SMListenerDef wires a pointer event on a named shape to one or more SM input actions.
type SMListenerDef struct {
	Target  string             `json:"target"`  // shape name
	Event   string             `json:"event"`   // "pointer_down"|"pointer_up"|"pointer_enter"|"pointer_exit"|"pointer_move"|"click"
	Actions []SMListenerAction `json:"actions"`
}

// SMListenerAction is one action executed when the listener fires.
type SMListenerAction struct {
	Type  string          `json:"type"`            // "set_bool"|"set_trigger"
	Input string          `json:"input"`           // input name
	Value json.RawMessage `json:"value,omitempty"` // required for set_bool: true or false
}

// SMInput is one state machine input.
type SMInput struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`            // "bool"|"number"|"trigger"
	Default json.RawMessage `json:"default,omitempty"`
}

// SMLayer is one state machine layer.
type SMLayer struct {
	Name        string         `json:"name"`
	States      []SMState      `json:"states"`
	Transitions []SMTransition `json:"transitions,omitempty"`
}

// SMBlendEntry is one animation threshold in a BlendState1D state.
type SMBlendEntry struct {
	Animation string  `json:"animation"`
	Threshold float64 `json:"threshold"`
}

// SMState is one state in a layer.
// Type: "animation" (default) uses the animation field.
// Type: "blend_1d" uses input + blends for BlendState1DInput.
type SMState struct {
	Name      string         `json:"name"`
	Type      string         `json:"type,omitempty"`       // "animation" | "blend_1d"
	Animation string         `json:"animation,omitempty"`  // for "animation" type
	Input     string         `json:"input,omitempty"`      // for "blend_1d" type
	Blends    []SMBlendEntry `json:"blends,omitempty"`     // for "blend_1d" type
}

// SMTransition is one transition between states.
// Use "ExitState" as From or To to target the layer sentinel.
type SMTransition struct {
	From       string        `json:"from"`
	To         string        `json:"to"`
	DurationMs int           `json:"duration_ms,omitempty"` // blend duration in milliseconds
	ExitTime   int           `json:"exit_time,omitempty"`   // frames before transition fires
	Conditions []SMCondition `json:"conditions,omitempty"`
}

// SMCondition is one transition condition.
type SMCondition struct {
	Input string          `json:"input"`
	Value json.RawMessage `json:"value,omitempty"`
	Op    string          `json:"op,omitempty"` // "=="|"!="|">"|"<"|">="|"<="
}

// ── ParseError ────────────────────────────────────────────────────────────────

// ParseError records a validation error with optional field context.
type ParseError struct {
	Field   string
	Message string
}

func (e *ParseError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidationWarning is a non-fatal diagnostic returned by ValidateJSON.
// Type-assert from the []error returned by ValidateJSON to distinguish from ParseError.
type ValidationWarning struct {
	Field   string
	Message string
}

func (w *ValidationWarning) Error() string {
	if w.Field != "" {
		return fmt.Sprintf("[WARN] %s: %s", w.Field, w.Message)
	}
	return fmt.Sprintf("[WARN] %s", w.Message)
}

// IsWarning reports whether err is a ValidationWarning.
func IsWarning(err error) bool {
	_, ok := err.(*ValidationWarning)
	return ok
}

// ── Public API ────────────────────────────────────────────────────────────────

// FromJSON parses a JSON scene description and returns a configured Builder.
func FromJSON(data []byte) (*builder.Builder, error) {
	var scene Scene
	if err := json.Unmarshal(data, &scene); err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if scene.Version != 1 {
		return nil, &ParseError{Field: "version", Message: fmt.Sprintf("unsupported version %d, want 1", scene.Version)}
	}
	return buildScene(&scene, "", nil)
}

// FromJSONFile reads path and builds a scene with font files resolved relative to path's directory.
func FromJSONFile(path string) (*builder.Builder, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var scene Scene
	if err := json.Unmarshal(data, &scene); err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if scene.Version != 1 {
		return nil, &ParseError{Field: "version", Message: fmt.Sprintf("unsupported version %d, want 1", scene.Version)}
	}
	return buildScene(&scene, filepath.Dir(path), nil)
}

// FromJSONWithFonts parses a JSON scene and resolves font file references from
// the provided bytes map (key = FontDef.File value in the JSON). Intended for
// testing where real font files are not available on disk.
func FromJSONWithFonts(data []byte, fonts map[string][]byte) (*builder.Builder, error) {
	var scene Scene
	if err := json.Unmarshal(data, &scene); err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if scene.Version != 1 {
		return nil, &ParseError{Field: "version", Message: fmt.Sprintf("unsupported version %d, want 1", scene.Version)}
	}
	return buildScene(&scene, "", fonts)
}

// ValidateJSON checks JSON structure without building the .riv.
// Returns a slice of errors (empty if valid).
func ValidateJSON(data []byte) []error {
	var errs []error

	var scene Scene
	if err := json.Unmarshal(data, &scene); err != nil {
		return []error{&ParseError{Message: fmt.Sprintf("invalid JSON syntax: %v", err)}}
	}

	if scene.Version != 1 {
		errs = append(errs, &ParseError{Field: "version", Message: fmt.Sprintf("unsupported version %d, want 1", scene.Version)})
	}

	ab := &scene.Artboard
	if ab.Name == "" {
		errs = append(errs, &ParseError{Field: "artboard.name", Message: "required"})
	}
	if ab.Width <= 0 {
		errs = append(errs, &ParseError{Field: "artboard.width", Message: "must be positive"})
	}
	if ab.Height <= 0 {
		errs = append(errs, &ParseError{Field: "artboard.height", Message: "must be positive"})
	}

	names := map[string]bool{}
	for i, child := range ab.Children {
		field := fmt.Sprintf("artboard.children[%d]", i)
		if child.Name == "" {
			errs = append(errs, &ParseError{Field: field + ".name", Message: "required"})
		} else if names[child.Name] {
			errs = append(errs, &ParseError{Field: field + ".name", Message: fmt.Sprintf("duplicate name %q", child.Name)})
		} else {
			names[child.Name] = true
		}
		switch strings.ToLower(child.Type) {
		case "rectangle", "ellipse":
			// no extra validation
		case "text":
			// text validation handled in buildScene
		case "path":
			if child.Closed && len(child.Vertices) < 3 {
				errs = append(errs, &ParseError{Field: field + ".vertices", Message: fmt.Sprintf("closed path requires at least 3 vertices, got %d", len(child.Vertices))})
			}
			for vi, v := range child.Vertices {
				vf := fmt.Sprintf("%s.vertices[%d]", field, vi)
				if (v.In != nil) != (v.Out != nil) {
					errs = append(errs, &ParseError{Field: vf, Message: "cubic vertex must have both 'in' and 'out' control points"})
				}
				if v.In != nil && len(v.In) != 2 {
					errs = append(errs, &ParseError{Field: vf + ".in", Message: "must be [x, y] (2 elements)"})
				}
				if v.Out != nil && len(v.Out) != 2 {
					errs = append(errs, &ParseError{Field: vf + ".out", Message: "must be [x, y] (2 elements)"})
				}
			}
		case "":
			errs = append(errs, &ParseError{Field: field + ".type", Message: "required"})
		default:
			errs = append(errs, &ParseError{Field: field + ".type", Message: fmt.Sprintf("unknown type %q (rectangle|ellipse|path|text)", child.Type)})
		}
	}

	// Validate clip references (second pass: all path names now collected)
	pathNames := map[string]bool{}
	for _, child := range ab.Children {
		if strings.ToLower(child.Type) == "path" && child.Name != "" {
			pathNames[child.Name] = true
		}
	}
	for i, child := range ab.Children {
		if child.Clip == "" {
			continue
		}
		if strings.ToLower(child.Type) == "path" {
			errs = append(errs, &ParseError{Field: fmt.Sprintf("artboard.children[%d].clip", i), Message: "path children cannot be clipped (only rectangle/ellipse)"})
		} else if !pathNames[child.Clip] {
			errs = append(errs, &ParseError{Field: fmt.Sprintf("artboard.children[%d].clip", i), Message: fmt.Sprintf("clip source %q not found or not a path", child.Clip)})
		}
	}

	animNames := map[string]bool{}
	for i, anim := range ab.Animations {
		field := fmt.Sprintf("artboard.animations[%d]", i)
		if anim.Name == "" {
			errs = append(errs, &ParseError{Field: field + ".name", Message: "required"})
		} else if animNames[anim.Name] {
			errs = append(errs, &ParseError{Field: field + ".name", Message: fmt.Sprintf("duplicate animation name %q", anim.Name)})
		} else {
			animNames[anim.Name] = true
		}
		if anim.Duration <= 0 {
			errs = append(errs, &ParseError{Field: field + ".duration", Message: "must be positive"})
		}
		for j, track := range anim.Tracks {
			tField := fmt.Sprintf("%s.tracks[%d]", field, j)
			if track.Target == "" {
				errs = append(errs, &ParseError{Field: tField + ".target", Message: "required"})
			} else if len(strings.Split(track.Target, ".")) < 2 {
				errs = append(errs, &ParseError{Field: tField + ".target", Message: "must be dot-path (e.g. shapeName.property)"})
			}
			for k, kf := range track.Keyframes {
				kfField := fmt.Sprintf("%s.keyframes[%d]", tField, k)
				if kf.Time < 0 {
					errs = append(errs, &ParseError{Field: kfField + ".time", Message: "must be >= 0"})
				}
				if kf.Value == nil {
					errs = append(errs, &ParseError{Field: kfField + ".value", Message: "required"})
				}
			}
		}
	}

	// SM-level structural checks
	smNames := map[string]bool{}
	for si, sm := range ab.StateMachines {
		sf := fmt.Sprintf("artboard.state_machines[%d]", si)
		if sm.Name == "" {
			errs = append(errs, &ParseError{Field: sf + ".name", Message: "required"})
			continue
		}
		if smNames[sm.Name] {
			errs = append(errs, &ParseError{Field: sf + ".name", Message: fmt.Sprintf("duplicate state machine name %q", sm.Name)})
		}
		smNames[sm.Name] = true
		if len(sm.Layers) == 0 {
			errs = append(errs, &ParseError{Field: fmt.Sprintf("%s[%q].layers", sf, sm.Name), Message: "at least one layer required"})
		}
		seenIns := map[string]bool{}
		for ii, inp := range sm.Inputs {
			if inp.Name == "" {
				errs = append(errs, &ParseError{Field: fmt.Sprintf("%s[%q].inputs[%d].name", sf, sm.Name, ii), Message: "required"})
			} else if seenIns[inp.Name] {
				errs = append(errs, &ParseError{Field: fmt.Sprintf("%s[%q].inputs", sf, sm.Name), Message: fmt.Sprintf("duplicate input name %q", inp.Name)})
			} else {
				seenIns[inp.Name] = true
			}
		}
		for li, layer := range sm.Layers {
			lf := fmt.Sprintf("%s[%q].layers[%d]", sf, sm.Name, li)
			seenSt := map[string]bool{}
			for sti, state := range layer.States {
				if state.Name == "" {
					errs = append(errs, &ParseError{Field: fmt.Sprintf("%s.states[%d].name", lf, sti), Message: "required"})
				} else if seenSt[state.Name] {
					errs = append(errs, &ParseError{Field: lf + ".states", Message: fmt.Sprintf("duplicate state name %q", state.Name)})
				} else {
					seenSt[state.Name] = true
				}
			}
			for ti, tr := range layer.Transitions {
				tf := fmt.Sprintf("%s.transitions[%d]", lf, ti)
				if tr.From == "" {
					errs = append(errs, &ParseError{Field: tf + ".from", Message: "required"})
				}
				if tr.To == "" {
					errs = append(errs, &ParseError{Field: tf + ".to", Message: "required"})
				}
			}
		}
		// Cycle detection: warn on back-edges in the transition graph (per layer)
		for _, layer := range sm.Layers {
			graph := map[string][]string{}
			for _, tr := range layer.Transitions {
				if tr.From != "" && tr.To != "" &&
					!strings.EqualFold(tr.From, "ExitState") &&
					!strings.EqualFold(tr.To, "ExitState") {
					graph[tr.From] = append(graph[tr.From], tr.To)
				}
			}
			visited := map[string]bool{}
			inStack := map[string]string{} // node → predecessor
			var dfs func(node, parent string) bool
			dfs = func(node, parent string) bool {
				visited[node] = true
				inStack[node] = parent
				for _, next := range graph[node] {
					if !visited[next] {
						if dfs(next, node) {
							return true
						}
					} else if _, onStack := inStack[next]; onStack {
						errs = append(errs, &ValidationWarning{
							Field:   fmt.Sprintf("state_machines[%q].layers[%q]", sm.Name, layer.Name),
							Message: fmt.Sprintf("cycle detected: %q → %q (transitions form a loop)", node, next),
						})
						return true
					}
				}
				delete(inStack, node)
				return false
			}
			for _, state := range layer.States {
				if !visited[state.Name] {
					dfs(state.Name, "")
				}
			}
		}
	}

	return errs
}

// ── Internal scene builder ────────────────────────────────────────────────────

func buildScene(scene *Scene, baseDir string, injectFonts map[string][]byte) (*builder.Builder, error) {
	ab := &scene.Artboard

	if ab.Name == "" {
		return nil, &ParseError{Field: "artboard.name", Message: "required"}
	}
	if ab.Width <= 0 || ab.Height <= 0 {
		return nil, &ParseError{Field: "artboard", Message: "width and height must be positive"}
	}

	b := builder.New()
	artboard := b.Artboard(ab.Name, ab.Width, ab.Height)

	// shapeMap: name → *ShapeRef for rect/ellipse children (draw rules, listeners, drawOrderKF)
	// pathMap:  name → *PathRef for path children (clip sources)
	// animMap:  name → AnimTarget for all children (animation targeting)
	shapeMap := map[string]*builder.ShapeRef{}
	pathMap  := map[string]*builder.PathRef{}
	animMap  := map[string]builder.AnimTarget{}

	// Load fonts and embed them in the artboard.
	fontMap := map[string]*builder.FontRef{}
	for i, fd := range ab.Fonts {
		ff := fmt.Sprintf("artboard.fonts[%d]", i)
		if fd.Name == "" {
			return nil, &ParseError{Field: ff + ".name", Message: "required"}
		}
		if fd.File == "" {
			return nil, &ParseError{Field: ff + ".file", Message: "required"}
		}
		var ttfBytes []byte
		if b, ok := injectFonts[fd.File]; ok {
			ttfBytes = b
		} else {
			if baseDir == "" {
				return nil, &ParseError{Field: ff + ".file", Message: "font file references require FromJSONFile (base directory unknown)"}
			}
			var err error
			ttfBytes, err = os.ReadFile(filepath.Join(baseDir, fd.File))
			if err != nil {
				return nil, &ParseError{Field: ff + ".file", Message: fmt.Sprintf("cannot read %q: %v", fd.File, err)}
			}
		}
		fontMap[fd.Name] = artboard.EmbedFont(fd.Name, ttfBytes)
	}

	for i, child := range ab.Children {
		cf := fmt.Sprintf("artboard.children[%d]", i)
		if child.Name == "" {
			return nil, &ParseError{Field: cf + ".name", Message: "required"}
		}
		if _, dup := animMap[child.Name]; dup {
			return nil, &ParseError{Field: cf + ".name", Message: fmt.Sprintf("duplicate name %q", child.Name)}
		}
		switch strings.ToLower(child.Type) {
		case "rectangle", "ellipse":
			ref, err := addChild(artboard, &child)
			if err != nil {
				return nil, err
			}
			shapeMap[child.Name] = ref
			animMap[child.Name] = ref
		case "path":
			ref, err := addPath(artboard, &child)
			if err != nil {
				return nil, err
			}
			pathMap[child.Name] = ref
			animMap[child.Name] = ref
		case "text":
			ref, err := addText(artboard, &child, fontMap)
			if err != nil {
				return nil, err
			}
			animMap[child.Name] = ref
		default:
			return nil, &ParseError{Field: cf + ".type", Message: fmt.Sprintf("unknown shape type %q (rectangle|ellipse|path|text)", child.Type)}
		}
	}

	// Resolve clipping: apply after all children added so pathMap is fully populated.
	for i, child := range ab.Children {
		if child.Clip == "" {
			continue
		}
		src, ok := shapeMap[child.Name]
		if !ok {
			return nil, &ParseError{Field: fmt.Sprintf("artboard.children[%d].clip", i), Message: "only rectangle/ellipse children can be clipped"}
		}
		clipPath, ok := pathMap[child.Clip]
		if !ok {
			return nil, &ParseError{Field: fmt.Sprintf("artboard.children[%d].clip", i), Message: fmt.Sprintf("clip source %q not found or not a path", child.Clip)}
		}
		src.ClipWith(clipPath)
	}

	// Add animations.
	animBuilders := []*builder.AnimationBuilder{}
	for _, anim := range ab.Animations {
		fps := anim.FPS
		if fps <= 0 {
			fps = 60
		}
		durationFrames := uint64(math.Round(anim.Duration * fps))
		if durationFrames == 0 {
			durationFrames = 1
		}
		speed := anim.Speed
		if speed == 0 {
			speed = 1.0
		}
		loop := parseLoopType(anim.Loop)

		ab2 := artboard.Animation(anim.Name,
			builder.WithDuration(durationFrames),
			builder.WithFPS(uint64(fps)),
			builder.WithLoop(loop),
			builder.WithSpeed(speed),
		)
		animBuilders = append(animBuilders, ab2)

		for ti, track := range anim.Tracks {
			tf := fmt.Sprintf("animations[%q].tracks[%d]", anim.Name, ti)
			ref, propKey, isColor, err := resolveTarget(track.Target, animMap)
			if err != nil {
				return nil, &ParseError{Field: tf + ".target", Message: err.Error()}
			}

			for ki, kf := range track.Keyframes {
				frame := uint64(math.Round(kf.Time * fps))
				interp, err := parseEasing(kf.Easing)
				if err != nil {
					return nil, &ParseError{
						Field:   fmt.Sprintf("%s.keyframes[%d].easing", tf, ki),
						Message: err.Error(),
					}
				}
				if isColor {
					var colorStr string
					if err := json.Unmarshal(kf.Value, &colorStr); err != nil {
						return nil, &ParseError{
							Field:   fmt.Sprintf("%s.keyframes[%d].value", tf, ki),
							Message: "color keyframe value must be a string (e.g. \"#FF0000\")",
						}
					}
					color, err := parseColor(colorStr)
					if err != nil {
						return nil, &ParseError{
							Field:   fmt.Sprintf("%s.keyframes[%d].value", tf, ki),
							Message: err.Error(),
						}
					}
					ab2.KeyframeColor(ref, propKey, frame, color, interp)
				} else {
					var val float64
					if err := json.Unmarshal(kf.Value, &val); err != nil {
						return nil, &ParseError{
							Field:   fmt.Sprintf("%s.keyframes[%d].value", tf, ki),
							Message: "float keyframe value must be a number",
						}
					}
					if propKey == builder.PropRotation {
						val = val * math.Pi / 180.0
					}
					ab2.KeyframeFloat(ref, propKey, frame, val, interp)
				}
			}
		}
	}
	// Process draw_order_tracks for each animation.
	for ai, anim := range ab.Animations {
		ab2 := animBuilders[ai]
		for ti, dot := range anim.DrawOrderTracks {
			dof := fmt.Sprintf("animations[%q].draw_order_tracks[%d]", anim.Name, ti)
			src, ok := shapeMap[dot.Shape]
			if !ok {
				return nil, &ParseError{
					Field:   dof + ".shape",
					Message: fmt.Sprintf("no shape named %q", dot.Shape),
				}
			}
			fps := anim.FPS
			if fps <= 0 {
				fps = 60
			}
			for ki, kf := range dot.Keyframes {
				frame := uint64(math.Round(kf.Time * fps))
				var targetRef *builder.ShapeRef
				var placement uint64
				if kf.Target != "" {
					t, ok2 := shapeMap[kf.Target]
					if !ok2 {
						return nil, &ParseError{
							Field:   fmt.Sprintf("%s.keyframes[%d].target", dof, ki),
							Message: fmt.Sprintf("no shape named %q", kf.Target),
						}
					}
					targetRef = t
					switch strings.ToLower(kf.Placement) {
					case "below":
						placement = builder.PlacementBelow
					default: // "above" or ""
						placement = builder.PlacementAbove
					}
				}
				ab2.KeyframeDrawOrder(src, frame, targetRef, placement)
			}
		}
	}
	_ = animBuilders

	// Apply draw rules (after all shapes added so targets are resolvable).
	for i, child := range ab.Children {
		if len(child.DrawRules) == 0 {
			continue
		}
		ref := shapeMap[child.Name]
		cf := fmt.Sprintf("artboard.children[%d].draw_rules", i)
		for j, rule := range child.DrawRules {
			target, ok := shapeMap[rule.Target]
			if !ok {
				return nil, &ParseError{
					Field:   fmt.Sprintf("%s[%d].target", cf, j),
					Message: fmt.Sprintf("no shape named %q", rule.Target),
				}
			}
			switch strings.ToLower(rule.Type) {
			case "above":
				ref.DrawAbove(target)
			case "below":
				ref.DrawBelow(target)
			default:
				return nil, &ParseError{
					Field:   fmt.Sprintf("%s[%d].type", cf, j),
					Message: fmt.Sprintf("unknown draw rule type %q (above|below)", rule.Type),
				}
			}
		}
	}

	// Add state machines.
	for _, sm := range ab.StateMachines {
		if sm.Name == "" {
			return nil, &ParseError{Field: "state_machines[].name", Message: "required"}
		}
		if len(sm.Layers) == 0 {
			return nil, &ParseError{
				Field:   fmt.Sprintf("state_machines[%q].layers", sm.Name),
				Message: "state machine must have at least one layer",
			}
		}

		smb := artboard.StateMachine(sm.Name)

		// Build input refs by name, tracking types for condition type checking.
		inputMap := map[string]*builder.InputRef{}
		inputTypeMap := map[string]string{} // name → "bool"|"number"|"trigger"
		seenInputs := map[string]bool{}
		for _, inp := range sm.Inputs {
			if inp.Name == "" {
				return nil, &ParseError{
					Field:   fmt.Sprintf("state_machines[%q].inputs[].name", sm.Name),
					Message: "required",
				}
			}
			if seenInputs[inp.Name] {
				return nil, &ParseError{
					Field:   fmt.Sprintf("state_machines[%q].inputs", sm.Name),
					Message: fmt.Sprintf("duplicate input name %q", inp.Name),
				}
			}
			seenInputs[inp.Name] = true
			inputTypeMap[inp.Name] = strings.ToLower(inp.Type)
		}

		for _, inp := range sm.Inputs {
			switch strings.ToLower(inp.Type) {
			case "bool":
				inputMap[inp.Name] = smb.BoolInput(inp.Name)
			case "number":
				ref := smb.NumberInput(inp.Name)
				if inp.Default != nil {
					var v float64
					if err := json.Unmarshal(inp.Default, &v); err != nil {
						return nil, &ParseError{
							Field:   fmt.Sprintf("state_machines[%q].inputs[%q].default", sm.Name, inp.Name),
							Message: "number input default must be a numeric value",
						}
					}
					ref.WithValue(v)
				}
				inputMap[inp.Name] = ref
			case "trigger":
				inputMap[inp.Name] = smb.TriggerInput(inp.Name)
			default:
				return nil, &ParseError{
					Field:   fmt.Sprintf("state_machines[%q].inputs[%q].type", sm.Name, inp.Name),
					Message: fmt.Sprintf("unknown input type %q (bool|number|trigger)", inp.Type),
				}
			}
		}

		for _, layer := range sm.Layers {
			lb := smb.Layer(layer.Name)

			stateMap := map[string]*builder.StateRef{}
			seenStates := map[string]bool{}
			for _, state := range layer.States {
				if state.Name == "" {
					return nil, &ParseError{
						Field:   fmt.Sprintf("state_machines[%q].layers[%q].states[].name", sm.Name, layer.Name),
						Message: "required",
					}
				}
				if seenStates[state.Name] {
					return nil, &ParseError{
						Field:   fmt.Sprintf("state_machines[%q].layers[%q].states", sm.Name, layer.Name),
						Message: fmt.Sprintf("duplicate state name %q", state.Name),
					}
				}
				seenStates[state.Name] = true

				switch strings.ToLower(state.Type) {
				case "blend_1d":
					// Input must exist and be numeric
					if state.Input == "" {
						return nil, &ParseError{
							Field:   fmt.Sprintf("state_machines[%q].layers[%q].states[%q].input", sm.Name, layer.Name, state.Name),
							Message: "blend_1d state requires an input name",
						}
					}
					if t, ok := inputTypeMap[state.Input]; ok && t != "number" {
						return nil, &ParseError{
							Field:   fmt.Sprintf("state_machines[%q].layers[%q].states[%q].input", sm.Name, layer.Name, state.Name),
							Message: fmt.Sprintf("blend_1d input %q must be type number, got %q", state.Input, t),
						}
					}
					inp, ok := inputMap[state.Input]
					if !ok {
						return nil, &ParseError{
							Field:   fmt.Sprintf("state_machines[%q].layers[%q].states[%q].input", sm.Name, layer.Name, state.Name),
							Message: fmt.Sprintf("unknown input %q", state.Input),
						}
					}
					// Thresholds must be non-decreasing
					for bi := 1; bi < len(state.Blends); bi++ {
						if state.Blends[bi].Threshold < state.Blends[bi-1].Threshold {
							return nil, &ParseError{
								Field:   fmt.Sprintf("state_machines[%q].layers[%q].states[%q].blends[%d].threshold", sm.Name, layer.Name, state.Name, bi),
								Message: fmt.Sprintf("thresholds must be non-decreasing: %.4g < %.4g", state.Blends[bi].Threshold, state.Blends[bi-1].Threshold),
							}
						}
					}
					br := lb.BlendState1D(state.Name, inp)
					for bi, ba := range state.Blends {
						if ba.Animation == "" {
							return nil, &ParseError{
								Field:   fmt.Sprintf("state_machines[%q].layers[%q].states[%q].blends[%d].animation", sm.Name, layer.Name, state.Name, bi),
								Message: "animation name required",
							}
						}
						br.AddAnimation(ba.Animation, ba.Threshold)
					}
					stateMap[state.Name] = br.StateHandle()
				default: // "animation" or ""
					var opts []builder.StateOption
					if state.Animation != "" {
						opts = append(opts, builder.WithAnimation(state.Animation))
					}
					stateMap[state.Name] = lb.State(state.Name, opts...)
				}
			}

			for _, trans := range layer.Transitions {
				lf := fmt.Sprintf("state_machines[%q].layers[%q].transitions", sm.Name, layer.Name)
				if trans.From == "" {
					return nil, &ParseError{Field: lf, Message: "transition.from is required"}
				}
				if trans.To == "" {
					return nil, &ParseError{Field: lf, Message: "transition.to is required"}
				}

				var from *builder.StateRef
				if strings.EqualFold(trans.From, "ExitState") {
					from = lb.ExitState()
				} else {
					var ok bool
					from, ok = stateMap[trans.From]
					if !ok {
						return nil, &ParseError{Field: lf, Message: fmt.Sprintf("unknown from state %q", trans.From)}
					}
				}

				var to *builder.StateRef
				if strings.EqualFold(trans.To, "ExitState") {
					to = lb.ExitState()
				} else {
					var ok bool
					to, ok = stateMap[trans.To]
					if !ok {
						return nil, &ParseError{Field: lf, Message: fmt.Sprintf("unknown to state %q", trans.To)}
					}
				}

				var conditions []builder.Condition
				for ci, cond := range trans.Conditions {
					cf := fmt.Sprintf("state_machines[%q].layers[%q].transitions[%q→%q].conditions[%d]", sm.Name, layer.Name, trans.From, trans.To, ci)
					inp, ok := inputMap[cond.Input]
					if !ok {
						return nil, &ParseError{
							Field:   cf + ".input",
							Message: fmt.Sprintf("unknown input %q", cond.Input),
						}
					}
					// Type mismatch: verify condition type matches input type
					inpType := inputTypeMap[cond.Input]
					if cond.Value == nil {
						// nil value → trigger condition; only valid on trigger inputs
						if inpType != "trigger" {
							return nil, &ParseError{
								Field:   cf,
								Message: fmt.Sprintf("trigger condition used on %q input %q (omit value only for trigger inputs)", inpType, cond.Input),
							}
						}
					} else {
						var boolVal bool
						if json.Unmarshal(cond.Value, &boolVal) == nil && inpType == "number" {
							return nil, &ParseError{
								Field:   cf + ".value",
								Message: fmt.Sprintf("bool condition value on number input %q; use a number and op instead", cond.Input),
							}
						}
						if inpType == "trigger" {
							return nil, &ParseError{
								Field:   cf,
								Message: fmt.Sprintf("condition value not allowed on trigger input %q; omit value for trigger conditions", cond.Input),
							}
						}
					}
					c, err := makeCondition(inp, &cond)
					if err != nil {
						return nil, err
					}
					conditions = append(conditions, c)
				}
				tr := lb.Transition(from, to, conditions...)
				if trans.DurationMs > 0 {
					tr.Duration(trans.DurationMs)
				}
				if trans.ExitTime > 0 {
					tr.ExitTime(trans.ExitTime)
				}
			}
		}

		// Listeners
		for li, ld := range sm.Listeners {
			lf := fmt.Sprintf("state_machines[%q].listeners[%d]", sm.Name, li)
			shapeRef, ok := shapeMap[ld.Target]
			if !ok {
				return nil, &ParseError{
					Field:   lf + ".target",
					Message: fmt.Sprintf("no shape named %q", ld.Target),
				}
			}
			et, err := parseListenerEvent(ld.Event)
			if err != nil {
				return nil, &ParseError{Field: lf + ".event", Message: err.Error()}
			}
			lr := smb.Listener(shapeRef, et)
			for ai, act := range ld.Actions {
				af := fmt.Sprintf("%s.actions[%d]", lf, ai)
				inp, ok := inputMap[act.Input]
				if !ok {
					return nil, &ParseError{
						Field:   af + ".input",
						Message: fmt.Sprintf("unknown input %q", act.Input),
					}
				}
				switch strings.ToLower(act.Type) {
				case "set_bool":
					var val bool
					if err := json.Unmarshal(act.Value, &val); err != nil {
						return nil, &ParseError{Field: af + ".value", Message: "must be true or false"}
					}
					lr.SetBool(inp, val)
				case "set_trigger":
					lr.SetTrigger(inp)
				case "set_number":
					var val float64
					if err := json.Unmarshal(act.Value, &val); err != nil {
						return nil, &ParseError{Field: af + ".value", Message: "must be a number"}
					}
					lr.SetNumber(inp, val)
				default:
					return nil, &ParseError{
						Field:   af + ".type",
						Message: fmt.Sprintf("unknown action type %q (set_bool|set_trigger|set_number)", act.Type),
					}
				}
			}
		}
	}

	return b, nil
}

// addChild adds one shape to the artboard and returns its ShapeRef.
func addChild(artboard *builder.ArtboardBuilder, child *Child) (*builder.ShapeRef, error) {
	var ref *builder.ShapeRef
	switch strings.ToLower(child.Type) {
	case "rectangle":
		ref = artboard.Rectangle(child.X, child.Y, child.Width, child.Height)
	case "ellipse":
		ref = artboard.Ellipse(child.X, child.Y, child.Width, child.Height)
	default:
		return nil, &ParseError{Field: "children[].type", Message: fmt.Sprintf("unknown shape type %q", child.Type)}
	}

	ref.Name(child.Name)

	if child.Rotation != 0 {
		ref.Rotation(child.Rotation)
	}
	if child.ScaleX != 0 || child.ScaleY != 0 {
		sx, sy := child.ScaleX, child.ScaleY
		if sx == 0 {
			sx = 1.0
		}
		if sy == 0 {
			sy = 1.0
		}
		ref.Scale(sx, sy)
	}
	if child.Opacity > 0 && child.Opacity < 1.0 {
		ref.Opacity(child.Opacity)
	}
	if child.CornerRadius > 0 && strings.ToLower(child.Type) == "rectangle" {
		ref.CornerRadius(child.CornerRadius)
	}

	if len(child.Fill) > 0 {
		if err := applyFill(ref, child.Fill); err != nil {
			return nil, err
		}
	}

	if child.Stroke != nil {
		color, err := parseColor(child.Stroke.Color)
		if err != nil {
			return nil, &ParseError{Field: "stroke.color", Message: err.Error()}
		}
		ref.Stroke(child.Stroke.Width, color)
	}

	return ref, nil
}

// addPath adds one custom path shape to the artboard and returns its PathRef.
func addPath(artboard *builder.ArtboardBuilder, child *Child) (*builder.PathRef, error) {
	if len(child.Vertices) == 0 {
		return nil, &ParseError{Field: fmt.Sprintf("children[%q].vertices", child.Name), Message: "path requires at least one vertex"}
	}
	if child.Closed && len(child.Vertices) < 3 {
		return nil, &ParseError{Field: fmt.Sprintf("children[%q].vertices", child.Name), Message: fmt.Sprintf("closed path requires at least 3 vertices, got %d", len(child.Vertices))}
	}

	ref := artboard.Path(child.X, child.Y)
	ref.Name(child.Name)

	for vi, v := range child.Vertices {
		vf := fmt.Sprintf("children[%q].vertices[%d]", child.Name, vi)
		if v.In != nil || v.Out != nil {
			// Cubic vertex: both in and out control points required
			if len(v.In) != 2 {
				return nil, &ParseError{Field: vf + ".in", Message: "cubic vertex requires in=[x,y] with exactly 2 elements"}
			}
			if len(v.Out) != 2 {
				return nil, &ParseError{Field: vf + ".out", Message: "cubic vertex requires out=[x,y] with exactly 2 elements"}
			}
			ref.CubicTo(v.X, v.Y, v.In[0], v.In[1], v.Out[0], v.Out[1])
		} else if v.Radius != 0 {
			ref.LineToR(v.X, v.Y, v.Radius)
		} else {
			ref.LineTo(v.X, v.Y)
		}
	}

	if child.Closed {
		ref.Close()
	}

	if child.Rotation != 0 {
		ref.Rotation(child.Rotation)
	}
	if child.ScaleX != 0 || child.ScaleY != 0 {
		sx, sy := child.ScaleX, child.ScaleY
		if sx == 0 {
			sx = 1.0
		}
		if sy == 0 {
			sy = 1.0
		}
		ref.Scale(sx, sy)
	}
	if child.Opacity > 0 && child.Opacity < 1.0 {
		ref.Opacity(child.Opacity)
	}

	if len(child.Fill) > 0 {
		if err := applyFillPath(ref, child.Fill); err != nil {
			return nil, err
		}
	}
	if child.Stroke != nil {
		color, err := parseColor(child.Stroke.Color)
		if err != nil {
			return nil, &ParseError{Field: fmt.Sprintf("children[%q].stroke.color", child.Name), Message: err.Error()}
		}
		ref.Stroke(child.Stroke.Width, color)
	}

	return ref, nil
}

// applyFillPath parses a fill JSON value and applies it to a PathRef.
func applyFillPath(ref *builder.PathRef, raw json.RawMessage) error {
	var colorStr string
	if err := json.Unmarshal(raw, &colorStr); err == nil {
		color, err := parseColor(colorStr)
		if err != nil {
			return &ParseError{Field: "fill", Message: err.Error()}
		}
		ref.Fill(color)
		return nil
	}

	var fill fillObj
	if err := json.Unmarshal(raw, &fill); err != nil {
		return &ParseError{Field: "fill", Message: fmt.Sprintf("invalid fill: must be \"#RRGGBB\" or {type, color, ...}: %v", err)}
	}

	switch strings.ToLower(fill.Type) {
	case "solid", "":
		if fill.Color == "" {
			return &ParseError{Field: "fill.color", Message: "required for solid fill"}
		}
		color, err := parseColor(fill.Color)
		if err != nil {
			return &ParseError{Field: "fill.color", Message: err.Error()}
		}
		ref.Fill(color)

	case "linear_gradient":
		if len(fill.Stops) < 2 {
			return &ParseError{Field: "fill.stops", Message: "linear_gradient requires at least 2 stops"}
		}
		stops := make([]builder.GradientStop, len(fill.Stops))
		for i, s := range fill.Stops {
			color, err := parseColor(s.Color)
			if err != nil {
				return &ParseError{Field: fmt.Sprintf("fill.stops[%d].color", i), Message: err.Error()}
			}
			stops[i] = builder.GradientStop{Position: s.Position, Color: color}
		}
		ref.FillGradient(fill.Start[0], fill.Start[1], fill.End[0], fill.End[1], stops...)

	case "radial_gradient":
		if len(fill.Stops) < 2 {
			return &ParseError{Field: "fill.stops", Message: "radial_gradient requires at least 2 stops"}
		}
		if fill.Radius <= 0 {
			return &ParseError{Field: "fill.radius", Message: "radial_gradient requires a positive radius"}
		}
		stops := make([]builder.GradientStop, len(fill.Stops))
		for i, s := range fill.Stops {
			color, err := parseColor(s.Color)
			if err != nil {
				return &ParseError{Field: fmt.Sprintf("fill.stops[%d].color", i), Message: err.Error()}
			}
			stops[i] = builder.GradientStop{Position: s.Position, Color: color}
		}
		cx, cy := fill.Center[0], fill.Center[1]
		ref.FillRadialGradient(cx, cy, cx+fill.Radius, cy, stops...)

	default:
		return &ParseError{Field: "fill.type", Message: fmt.Sprintf("unknown fill type %q (solid|linear_gradient|radial_gradient)", fill.Type)}
	}
	return nil
}

// applyFill parses a fill JSON value (string "#RRGGBB" or object) and applies it to ref.
func applyFill(ref *builder.ShapeRef, raw json.RawMessage) error {
	// Try string shorthand: "#RRGGBB"
	var colorStr string
	if err := json.Unmarshal(raw, &colorStr); err == nil {
		color, err := parseColor(colorStr)
		if err != nil {
			return &ParseError{Field: "fill", Message: err.Error()}
		}
		ref.Fill(color)
		return nil
	}

	// Try object form
	var fill fillObj
	if err := json.Unmarshal(raw, &fill); err != nil {
		return &ParseError{Field: "fill", Message: fmt.Sprintf("invalid fill: must be \"#RRGGBB\" or {type, color, ...}: %v", err)}
	}

	switch strings.ToLower(fill.Type) {
	case "solid", "":
		if fill.Color == "" {
			return &ParseError{Field: "fill.color", Message: "required for solid fill"}
		}
		color, err := parseColor(fill.Color)
		if err != nil {
			return &ParseError{Field: "fill.color", Message: err.Error()}
		}
		ref.Fill(color)

	case "linear_gradient":
		if len(fill.Stops) < 2 {
			return &ParseError{Field: "fill.stops", Message: "linear_gradient requires at least 2 stops"}
		}
		stops := make([]builder.GradientStop, len(fill.Stops))
		for i, s := range fill.Stops {
			color, err := parseColor(s.Color)
			if err != nil {
				return &ParseError{Field: fmt.Sprintf("fill.stops[%d].color", i), Message: err.Error()}
			}
			stops[i] = builder.GradientStop{Position: s.Position, Color: color}
		}
		ref.FillGradient(fill.Start[0], fill.Start[1], fill.End[0], fill.End[1], stops...)

	case "radial_gradient":
		if len(fill.Stops) < 2 {
			return &ParseError{Field: "fill.stops", Message: "radial_gradient requires at least 2 stops"}
		}
		if fill.Radius <= 0 {
			return &ParseError{Field: "fill.radius", Message: "radial_gradient requires a positive radius"}
		}
		stops := make([]builder.GradientStop, len(fill.Stops))
		for i, s := range fill.Stops {
			color, err := parseColor(s.Color)
			if err != nil {
				return &ParseError{Field: fmt.Sprintf("fill.stops[%d].color", i), Message: err.Error()}
			}
			stops[i] = builder.GradientStop{Position: s.Position, Color: color}
		}
		// center + (radius, 0) as edge point — distance from center to edge = radius
		cx, cy := fill.Center[0], fill.Center[1]
		ref.FillRadialGradient(cx, cy, cx+fill.Radius, cy, stops...)

	default:
		return &ParseError{Field: "fill.type", Message: fmt.Sprintf("unknown fill type %q (solid|linear_gradient|radial_gradient)", fill.Type)}
	}
	return nil
}

// parseListenerEvent maps a JSON event string to a builder.ListenerType.
func parseListenerEvent(event string) (builder.ListenerType, error) {
	switch strings.ToLower(event) {
	case "pointer_down":
		return builder.ListenerPointerDown, nil
	case "pointer_up":
		return builder.ListenerPointerUp, nil
	case "pointer_enter":
		return builder.ListenerPointerEnter, nil
	case "pointer_exit":
		return builder.ListenerPointerExit, nil
	case "pointer_move":
		return builder.ListenerPointerMove, nil
	case "click":
		return builder.ListenerClick, nil
	default:
		return 0, fmt.Errorf("unknown event %q (pointer_down|pointer_up|pointer_enter|pointer_exit|pointer_move|click)", event)
	}
}

// resolveTarget maps a dot-path target string to the AnimTarget and property info.
// Accepts animMap (all shapes + paths). For vertex animation use "name.vN.x" form.
func resolveTarget(path string, names map[string]builder.AnimTarget) (ref builder.AnimTarget, propKey uint32, isColor bool, err error) {
	dot := strings.Index(path, ".")
	if dot < 0 {
		return nil, 0, false, fmt.Errorf("expected dot-path (e.g. shapeName.x), got %q", path)
	}
	shapeName := path[:dot]
	prop := path[dot+1:]

	ref, ok := names[shapeName]
	if !ok {
		// Suggest names
		var available []string
		for k := range names {
			available = append(available, k)
		}
		return nil, 0, false, fmt.Errorf("no shape named %q (available: %s)", shapeName, strings.Join(available, ", "))
	}

	// Text-specific sub-paths: "style.fontSize", "style.fill.color", etc.
	if textRef, ok := ref.(*builder.TextRef); ok && strings.HasPrefix(prop, "style.") {
		styleProp := prop[len("style."):]
		style := textRef.FirstStyle()
		if style == nil {
			return nil, 0, false, fmt.Errorf("text %q has no styles defined", shapeName)
		}
		switch styleProp {
		case "fontSize":
			return style, builder.PropFontSize, false, nil
		case "letterSpacing":
			return style, builder.PropLetterSpacing, false, nil
		case "lineHeight":
			return style, builder.PropLineHeight, false, nil
		case "fill.color":
			return style, builder.PropColorValue, true, nil
		default:
			return nil, 0, false, fmt.Errorf("unknown text style property %q (supported: fontSize, letterSpacing, lineHeight, fill.color)", styleProp)
		}
	}

	switch prop {
	case "x":
		return ref, builder.PropX, false, nil
	case "y":
		return ref, builder.PropY, false, nil
	case "rotation":
		return ref, builder.PropRotation, false, nil
	case "scaleX":
		return ref, builder.PropScaleX, false, nil
	case "scaleY":
		return ref, builder.PropScaleY, false, nil
	case "opacity":
		return ref, builder.PropOpacity, false, nil
	case "fill.color":
		return ref, builder.PropColorValue, true, nil
	default:
		supported := "x, y, rotation, scaleX, scaleY, opacity, fill.color, style.fontSize, style.fill.color, style.letterSpacing, style.lineHeight"
		return nil, 0, false, fmt.Errorf("unknown property %q (supported: %s)", prop, supported)
	}
}

// makeCondition builds a builder.Condition from an SMCondition.
func makeCondition(inp *builder.InputRef, cond *SMCondition) (builder.Condition, error) {
	if cond.Value == nil {
		return builder.TriggerCondition(inp), nil
	}

	var boolVal bool
	if err := json.Unmarshal(cond.Value, &boolVal); err == nil {
		return builder.BoolCondition(inp, boolVal), nil
	}

	var numVal float64
	if err := json.Unmarshal(cond.Value, &numVal); err == nil {
		op := parseCompareOp(cond.Op)
		return builder.NumberCondition(inp, op, numVal), nil
	}

	return nil, &ParseError{Message: fmt.Sprintf("condition value for input %q must be bool or number", cond.Input)}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// parseColor parses #RGB, #RRGGBB (→0xFFRRGGBB), or #AARRGGBB.
func parseColor(s string) (uint32, error) {
	if len(s) == 0 || s[0] != '#' {
		return 0, fmt.Errorf("invalid color %q: must start with #", s)
	}
	hex := s[1:]
	switch len(hex) {
	case 3: // #RGB → #RRGGBB
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		fallthrough
	case 6: // #RRGGBB → 0xFFRRGGBB
		v, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid color %q", s)
		}
		return 0xFF000000 | uint32(v), nil
	case 8: // #AARRGGBB
		v, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid color %q", s)
		}
		return uint32(v), nil
	default:
		return 0, fmt.Errorf("invalid color %q: expected #RGB, #RRGGBB, or #AARRGGBB", s)
	}
}

// parseLoopType maps loop string to builder.LoopType.
func parseLoopType(s string) builder.LoopType {
	switch strings.ToLower(s) {
	case "loop":
		return builder.Loop
	case "pingpong", "ping-pong":
		return builder.PingPong
	default: // "oneshot" or ""
		return builder.OneShot
	}
}

// parseEasing parses an easing value (string preset or cubic object).
func parseEasing(raw json.RawMessage) (builder.Interpolation, error) {
	if raw == nil {
		return builder.Linear(), nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "linear", "":
			return builder.Linear(), nil
		case "hold":
			return builder.Hold(), nil
		case "ease-in":
			return builder.Cubic(0.42, 0, 1, 1), nil
		case "ease-out":
			return builder.Cubic(0, 0, 0.58, 1), nil
		case "ease-in-out":
			return builder.Cubic(0.42, 0, 0.58, 1), nil
		default:
			return nil, fmt.Errorf("unknown easing %q (linear|hold|ease-in|ease-out|ease-in-out)", s)
		}
	}

	var cubic struct {
		X1 float64 `json:"x1"`
		Y1 float64 `json:"y1"`
		X2 float64 `json:"x2"`
		Y2 float64 `json:"y2"`
	}
	if err := json.Unmarshal(raw, &cubic); err != nil {
		return nil, fmt.Errorf("easing must be a preset string or cubic bezier object {x1,y1,x2,y2}")
	}
	return builder.Cubic(cubic.X1, cubic.Y1, cubic.X2, cubic.Y2), nil
}

// parseCompareOp maps op string to builder.CompareOp.
func parseCompareOp(op string) builder.CompareOp {
	switch op {
	case "!=":
		return builder.NotEqual
	case "<":
		return builder.LessThan
	case ">":
		return builder.GreaterThan
	case "<=":
		return builder.LessThanOrEqual
	case ">=":
		return builder.GreaterThanOrEqual
	default: // "==" or ""
		return builder.Equal
	}
}

// ── Text support ──────────────────────────────────────────────────────────────

// addText adds a text child to artboard using fontMap to resolve font references.
// Supports two formats:
//   - Single-run: child.Style + child.Text (legacy, backward-compatible)
//   - Multi-run:  child.Styles + child.Runs
func addText(artboard *builder.ArtboardBuilder, child *Child, fontMap map[string]*builder.FontRef) (*builder.TextRef, error) {
	cf := fmt.Sprintf("child %q", child.Name)

	ref := artboard.Text(child.Name).
		Position(child.X, child.Y).
		Align(parseTextAlign(child.Align)).
		Overflow(parseTextOverflow(child.Overflow))

	switch strings.ToLower(child.Sizing) {
	case "auto_height":
		ref.Sizing(builder.SizingAutoHeight)
	case "fixed":
		ref.Sizing(builder.SizingFixed).Size(child.Width, child.Height)
	}

	if len(child.Runs) > 0 {
		// ── Multi-run format ─────────────────────────────────────────────────
		if len(child.Styles) == 0 {
			return nil, &ParseError{Field: cf + ".styles", Message: "styles array required when runs is set"}
		}

		styleRefs := make(map[string]*builder.TextStyleRef, len(child.Styles))
		for i, sd := range child.Styles {
			sf := fmt.Sprintf("%s.styles[%d]", cf, i)
			if sd.Name == "" {
				return nil, &ParseError{Field: sf + ".name", Message: "required"}
			}
			if sd.Font == "" {
				return nil, &ParseError{Field: sf + ".font", Message: "required"}
			}
			if sd.FontSize <= 0 {
				return nil, &ParseError{Field: sf + ".fontSize", Message: "must be > 0"}
			}
			font, ok := fontMap[sd.Font]
			if !ok {
				return nil, &ParseError{Field: sf + ".font", Message: fmt.Sprintf("font %q not defined in artboard.fonts", sd.Font)}
			}
			s := ref.Style(font, sd.FontSize)
			if sd.Fill != "" {
				color, err := parseColor(sd.Fill)
				if err != nil {
					return nil, &ParseError{Field: sf + ".fill", Message: err.Error()}
				}
				s.Fill(color)
			}
			if sd.LineHeight != 0 {
				s.LineHeight(sd.LineHeight)
			}
			if sd.LetterSpacing != 0 {
				s.LetterSpacing(sd.LetterSpacing)
			}
			styleRefs[sd.Name] = s
		}

		for i, run := range child.Runs {
			rf := fmt.Sprintf("%s.runs[%d]", cf, i)
			if run.Text == "" {
				return nil, &ParseError{Field: rf + ".text", Message: "required"}
			}
			if run.Style == "" {
				return nil, &ParseError{Field: rf + ".style", Message: "required"}
			}
			s, ok := styleRefs[run.Style]
			if !ok {
				return nil, &ParseError{Field: rf + ".style", Message: fmt.Sprintf("style %q not declared in styles array", run.Style)}
			}
			ref.Run(run.Text, s)
		}

		return ref, nil
	}

	// ── Single-run format (backward-compatible) ───────────────────────────────
	if child.Text == "" {
		return nil, &ParseError{Field: cf + ".text", Message: "text content required for type=text (use 'runs' for multi-run)"}
	}
	if child.Style == nil {
		return nil, &ParseError{Field: cf + ".style", Message: "style required for type=text"}
	}
	if child.Style.Font == "" {
		return nil, &ParseError{Field: cf + ".style.font", Message: "font name required"}
	}
	if child.Style.FontSize <= 0 {
		return nil, &ParseError{Field: cf + ".style.fontSize", Message: "fontSize must be > 0"}
	}

	font, ok := fontMap[child.Style.Font]
	if !ok {
		return nil, &ParseError{Field: cf + ".style.font", Message: fmt.Sprintf("font %q not defined in artboard.fonts", child.Style.Font)}
	}

	style := ref.Style(font, child.Style.FontSize)
	if child.Style.Fill != "" {
		color, err := parseColor(child.Style.Fill)
		if err != nil {
			return nil, &ParseError{Field: cf + ".style.fill", Message: err.Error()}
		}
		style.Fill(color)
	}
	if child.Style.LineHeight != 0 {
		style.LineHeight(child.Style.LineHeight)
	}
	if child.Style.LetterSpacing != 0 {
		style.LetterSpacing(child.Style.LetterSpacing)
	}

	ref.Run(child.Text, style)
	return ref, nil
}

// parseTextAlign maps an align string to builder.TextAlign.
func parseTextAlign(s string) builder.TextAlign {
	switch strings.ToLower(s) {
	case "right":
		return builder.AlignRight
	case "center":
		return builder.AlignCenter
	default: // "left" or ""
		return builder.AlignLeft
	}
}

// parseTextOverflow maps an overflow string to builder.TextOverflow.
func parseTextOverflow(s string) builder.TextOverflow {
	switch strings.ToLower(s) {
	case "hidden":
		return builder.OverflowHidden
	case "clipped":
		return builder.OverflowClipped
	case "ellipsis":
		return builder.OverflowEllipsis
	case "fit":
		return builder.OverflowFit
	default: // "visible" or ""
		return builder.OverflowVisible
	}
}
