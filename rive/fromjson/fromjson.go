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

// Artboard describes the canvas and its children.
type Artboard struct {
	Name          string         `json:"name"`
	Width         float64        `json:"width"`
	Height        float64        `json:"height"`
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
	return buildScene(&scene)
}

// FromJSONFile reads path and calls FromJSON.
func FromJSONFile(path string) (*builder.Builder, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return FromJSON(data)
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
		case "":
			errs = append(errs, &ParseError{Field: field + ".type", Message: "required"})
		default:
			errs = append(errs, &ParseError{Field: field + ".type", Message: fmt.Sprintf("unknown type %q (rectangle|ellipse)", child.Type)})
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

	return errs
}

// ── Internal scene builder ────────────────────────────────────────────────────

func buildScene(scene *Scene) (*builder.Builder, error) {
	ab := &scene.Artboard

	if ab.Name == "" {
		return nil, &ParseError{Field: "artboard.name", Message: "required"}
	}
	if ab.Width <= 0 || ab.Height <= 0 {
		return nil, &ParseError{Field: "artboard", Message: "width and height must be positive"}
	}

	b := builder.New()
	artboard := b.Artboard(ab.Name, ab.Width, ab.Height)

	// Add shapes, tracking name → ShapeRef for animation targeting.
	nameMap := map[string]*builder.ShapeRef{}
	for i, child := range ab.Children {
		cf := fmt.Sprintf("artboard.children[%d]", i)
		if child.Name == "" {
			return nil, &ParseError{Field: cf + ".name", Message: "required"}
		}
		if _, dup := nameMap[child.Name]; dup {
			return nil, &ParseError{Field: cf + ".name", Message: fmt.Sprintf("duplicate name %q", child.Name)}
		}
		ref, err := addChild(artboard, &child)
		if err != nil {
			return nil, err
		}
		nameMap[child.Name] = ref
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
			ref, propKey, isColor, err := resolveTarget(track.Target, nameMap)
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
			src, ok := nameMap[dot.Shape]
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
					t, ok2 := nameMap[kf.Target]
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
		ref := nameMap[child.Name]
		cf := fmt.Sprintf("artboard.children[%d].draw_rules", i)
		for j, rule := range child.DrawRules {
			target, ok := nameMap[rule.Target]
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
		smb := artboard.StateMachine(sm.Name)

		// Build input refs by name.
		inputMap := map[string]*builder.InputRef{}
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
			for _, state := range layer.States {
				switch strings.ToLower(state.Type) {
				case "blend_1d":
					inp, ok := inputMap[state.Input]
					if !ok {
						return nil, &ParseError{
							Field:   fmt.Sprintf("state_machines[%q].layers[%q].states[%q].input", sm.Name, layer.Name, state.Name),
							Message: fmt.Sprintf("unknown input %q", state.Input),
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
				for _, cond := range trans.Conditions {
					inp, ok := inputMap[cond.Input]
					if !ok {
						return nil, &ParseError{
							Field:   fmt.Sprintf("state_machines[%q] condition", sm.Name),
							Message: fmt.Sprintf("unknown input %q", cond.Input),
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
			shapeRef, ok := nameMap[ld.Target]
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

// resolveTarget maps a dot-path target string to the ShapeRef and property info.
func resolveTarget(path string, names map[string]*builder.ShapeRef) (ref *builder.ShapeRef, propKey uint32, isColor bool, err error) {
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
		supported := "x, y, rotation, scaleX, scaleY, opacity, fill.color"
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
