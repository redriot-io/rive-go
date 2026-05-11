package builder

import (
	"math"
	"sort"

	"github.com/redriot-io/rive-go/rive"
)

// LoopType controls how a linear animation repeats.
type LoopType uint64

const (
	OneShot  LoopType = 0
	Loop     LoopType = 1
	PingPong LoopType = 2
)

// Interpolation specifies the easing between two keyframes.
type Interpolation interface {
	interpTypeCode() uint64
}

// LinearInterp is a linear (constant-velocity) interpolation.
type LinearInterp struct{}

// interpTypeCode: Rive spec defines 0=hold, 1=linear, 2=cubic.
// The generated InterpolatingKeyFrame.Properties() suppresses emission when value==0 (hold is default).
func (LinearInterp) interpTypeCode() uint64 { return 1 }

// HoldInterp is a discrete/hold interpolation (no blending).
type HoldInterp struct{}

func (HoldInterp) interpTypeCode() uint64 { return 0 }

// CubicInterp is a cubic Bezier interpolation defined by two control points.
type CubicInterp struct{ X1, Y1, X2, Y2 float64 }

func (CubicInterp) interpTypeCode() uint64 { return 2 }

// Linear returns a linear interpolation.
func Linear() Interpolation { return LinearInterp{} }

// Hold returns a hold/discrete interpolation.
func Hold() Interpolation { return HoldInterp{} }

// Cubic returns a cubic Bezier interpolation.
func Cubic(x1, y1, x2, y2 float64) Interpolation { return CubicInterp{x1, y1, x2, y2} }

// AnimationOption configures a linear animation.
type AnimationOption func(*animConfig)

type animConfig struct {
	fps      uint64
	duration uint64
	speed    float64
	loop     LoopType
}

// WithFPS sets the frames-per-second rate (default 60).
func WithFPS(fps uint64) AnimationOption { return func(c *animConfig) { c.fps = fps } }

// WithDuration sets the animation length in frames (default 60).
func WithDuration(frames uint64) AnimationOption { return func(c *animConfig) { c.duration = frames } }

// WithLoop sets the loop type.
func WithLoop(l LoopType) AnimationOption { return func(c *animConfig) { c.loop = l } }

// WithSpeed sets playback speed multiplier (default 1.0).
func WithSpeed(s float64) AnimationOption { return func(c *animConfig) { c.speed = s } }

// AnimTarget is implemented by any animation target that exposes its artboard-relative
// object index. *ShapeRef, *PathRef, and *PathVertexRef all implement this interface,
// so they can all be passed to KeyframeFloat / KeyframeColor.
type AnimTarget interface {
	animIdx() uint64
	animColorIdx() (uint64, bool)
}

// animKF is one keyframe entry stored by AnimationBuilder.
type animKF struct {
	target  AnimTarget
	propKey uint32
	frame   uint64
	value   float64 // float keyframe value
	color   uint32  // color keyframe value
	isColor bool
	interp  Interpolation
}

// drawOrderKF is one animated draw-order keyframe.
// It switches DrawRules.DrawTargetId at a given frame (always hold).
// target=nil means "reset to default order" (DrawTargetId = sentinel).
type drawOrderKF struct {
	source    *ShapeRef
	frame     uint64
	target    *ShapeRef // nil = no active rule
	placement uint64
}

// AnimationBuilder constructs a LinearAnimation with its keyed targets.
type AnimationBuilder struct {
	name         string
	fps          uint64
	duration     uint64
	speed        float64
	loop         LoopType
	kfs          []animKF
	drawOrderKFs []drawOrderKF
	idx          uint64 // global emission index, set during emit()
}

func newAnimationBuilder(name string, opts ...AnimationOption) *AnimationBuilder {
	cfg := &animConfig{fps: 60, duration: 60, speed: 1.0}
	for _, o := range opts {
		o(cfg)
	}
	return &AnimationBuilder{
		name:     name,
		fps:      cfg.fps,
		duration: cfg.duration,
		speed:    cfg.speed,
		loop:     cfg.loop,
	}
}

// KeyframeFloat adds a float keyframe for x, y, opacity, rotation, scale, etc.
func (a *AnimationBuilder) KeyframeFloat(target AnimTarget, propKey uint32, frame uint64, value float64, interp ...Interpolation) *AnimationBuilder {
	i := Interpolation(LinearInterp{})
	if len(interp) > 0 {
		i = interp[0]
	}
	a.kfs = append(a.kfs, animKF{target: target, propKey: propKey, frame: frame, value: value, interp: i})
	return a
}

// KeyframeRotation adds a rotation keyframe, accepting degrees (converted to radians internally).
func (a *AnimationBuilder) KeyframeRotation(target AnimTarget, frame uint64, degrees float64, interp ...Interpolation) *AnimationBuilder {
	return a.KeyframeFloat(target, PropRotation, frame, degrees*math.Pi/180.0, interp...)
}

// KeyframeColor adds a color keyframe.
func (a *AnimationBuilder) KeyframeColor(target AnimTarget, propKey uint32, frame uint64, color uint32, interp ...Interpolation) *AnimationBuilder {
	i := Interpolation(LinearInterp{})
	if len(interp) > 0 {
		i = interp[0]
	}
	a.kfs = append(a.kfs, animKF{target: target, propKey: propKey, frame: frame, color: color, isColor: true, interp: i})
	return a
}

// KeyframeDrawOrder adds a hold keyframe that switches a shape's draw order at a
// specific frame. source is the shape whose animated DrawRules object will be keyed;
// target is the reference shape to draw relative to; placement is PlacementAbove or
// PlacementBelow.
//
// Passing target=nil emits a sentinel KeyFrameId (Value=^uint64(0), Rive's -1),
// which deactivates the DrawRules so the source falls back to default hierarchy order.
//
// All draw-order keyframes are implicitly hold (InterpolationType=0); interpolation
// is not meaningful for ID-typed properties. Returns a for chaining.
func (a *AnimationBuilder) KeyframeDrawOrder(source *ShapeRef, frame uint64, target *ShapeRef, placement uint64) *AnimationBuilder {
	a.drawOrderKFs = append(a.drawOrderKFs, drawOrderKF{
		source:    source,
		frame:     frame,
		target:    target,
		placement: placement,
	})
	return a
}

// cubicKey is used to deduplicate CubicEaseInterpolator objects.
type cubicKey struct{ x1, y1, x2, y2 float64 }

// doTargetKey uniquely identifies a (target shape, placement) pair.
type doTargetKey struct{ shapeIdx, placement uint64 }

// doSlot tracks the DrawRules object and its associated DrawTarget objects
// emitted for one source shape's animated draw order.
type doSlot struct {
	rulesIdx uint64              // artboard-relative index of the DrawRules object
	dtMap    map[doTargetKey]uint64 // (targetShapeIdx, placement) → DrawTarget artboard-relative index
}

// emit writes the LinearAnimation object graph into the slice.
// Must be called after all ShapeRef.emitObjects() so that ShapeRef indices are set.
func (a *AnimationBuilder) emit(objects *[]rive.Object, artboardOffset uint64) error {
	// Collect unique cubic Bezier curves used by this animation's keyframes.
	// Emit a CubicEaseInterpolator object for each unique curve BEFORE the
	// LinearAnimation so that the global object indices are stable.
	type curveEntry struct {
		key cubicKey
		idx uint64
	}
	curveMap := map[cubicKey]uint64{}
	var curveOrder []cubicKey

	for _, kf := range a.kfs {
		if ci, ok := kf.interp.(CubicInterp); ok {
			ck := cubicKey{ci.X1, ci.Y1, ci.X2, ci.Y2}
			if _, seen := curveMap[ck]; !seen {
				curveMap[ck] = uint64(len(*objects)) - artboardOffset
				curveOrder = append(curveOrder, ck)
				ce := &rive.CubicEaseInterpolator{}
				ce.X1 = ci.X1
				ce.Y1 = ci.Y1
				ce.X2 = ci.X2
				ce.Y2 = ci.Y2
				*objects = append(*objects, ce)
			}
		}
	}

	// ── Animated draw order pre-pass ─────────────────────────────────────────
	// Emit DrawTarget + DrawRules objects for animated draw order keyframes
	// BEFORE the LinearAnimation so their artboard-relative indices are known
	// when emitting the animation's KeyFrameId objects.
	slotMap := map[*ShapeRef]*doSlot{}
	var slotOrder []*ShapeRef

	for _, kf := range a.drawOrderKFs {
		if _, ok := slotMap[kf.source]; !ok {
			slotMap[kf.source] = &doSlot{dtMap: map[doTargetKey]uint64{}}
			slotOrder = append(slotOrder, kf.source)
		}
	}

	for _, src := range slotOrder {
		slot := slotMap[src]

		// Emit DrawRules first so DrawTarget objects can reference it as parent.
		// DrawTargetId is sentinel (^uint64(0)) — animation keyframes drive it.
		slot.rulesIdx = uint64(len(*objects)) - artboardOffset
		dr := &rive.DrawRules{}
		dr.ParentId = src.shapeIdx
		dr.DrawTargetId = ^uint64(0)
		*objects = append(*objects, dr)

		// Emit one DrawTarget per unique (target shape, placement) pair as children.
		for _, kf := range a.drawOrderKFs {
			if kf.source != src || kf.target == nil {
				continue
			}
			k := doTargetKey{kf.target.shapeIdx, kf.placement}
			if _, exists := slot.dtMap[k]; !exists {
				dtIdx := uint64(len(*objects)) - artboardOffset
				dt := &rive.DrawTarget{}
				dt.ParentId = slot.rulesIdx // child of DrawRules, matching Rive editor output
				dt.DrawableId = kf.target.shapeIdx
				dt.PlacementValue = kf.placement
				*objects = append(*objects, dt)
				slot.dtMap[k] = dtIdx
			}
		}
	}

	// LinearAnimation follows the interpolator objects.
	a.idx = uint64(len(*objects))
	la := &rive.LinearAnimation{}
	la.Name = a.name
	la.Fps = a.fps
	la.Duration = a.duration
	la.Speed = a.speed
	la.LoopValue = uint64(a.loop)
	// Sentinel values suppress emission — Go zero (0) would be encoded as a
	// 0-frame work area, causing the runtime to compute 0-duration playback.
	la.WorkStart = ^uint64(0)
	la.WorkEnd = ^uint64(0)
	*objects = append(*objects, la)

	// Group keyframes by target object and property key.
	type tpKey struct {
		objIdx  uint64
		propKey uint32
	}
	type group struct {
		objIdx  uint64
		propKey uint32
		kfs     []animKF
	}

	groupMap := map[tpKey]*group{}
	var groupOrder []tpKey

	for _, kf := range a.kfs {
		var objIdx uint64
		if kf.isColor {
			if colorIdx, ok := kf.target.animColorIdx(); ok {
				objIdx = colorIdx
			} else {
				objIdx = kf.target.animIdx()
			}
		} else {
			objIdx = kf.target.animIdx()
		}
		k := tpKey{objIdx, kf.propKey}
		if _, ok := groupMap[k]; !ok {
			groupMap[k] = &group{objIdx: objIdx, propKey: kf.propKey}
			groupOrder = append(groupOrder, k)
		}
		groupMap[k].kfs = append(groupMap[k].kfs, kf)
	}

	// Collect unique object indices (preserving first-seen order).
	objMap := map[uint64][]tpKey{}
	var objOrder []uint64
	seenObj := map[uint64]bool{}
	for _, k := range groupOrder {
		if !seenObj[k.objIdx] {
			seenObj[k.objIdx] = true
			objOrder = append(objOrder, k.objIdx)
		}
		objMap[k.objIdx] = append(objMap[k.objIdx], k)
	}

	for _, objIdx := range objOrder {
		kobj := &rive.KeyedObject{}
		kobj.ObjectId = objIdx
		*objects = append(*objects, kobj)

		for _, k := range objMap[objIdx] {
			g := groupMap[k]
			kprop := &rive.KeyedProperty{}
			kprop.PropertyKey = uint64(g.propKey)
			*objects = append(*objects, kprop)

			// Sort frames by frame number before emitting.
			sort.Slice(g.kfs, func(i, j int) bool { return g.kfs[i].frame < g.kfs[j].frame })

			for _, kf := range g.kfs {
				it := kf.interp.interpTypeCode()

				// Resolve interpolator ID for cubic easing.
				interpID := ^uint64(0) // sentinel = not emitted (linear/hold)
				if ci, ok := kf.interp.(CubicInterp); ok {
					ck := cubicKey{ci.X1, ci.Y1, ci.X2, ci.Y2}
					if idx, found := curveMap[ck]; found {
						interpID = idx
					}
				}

				if kf.isColor {
					f := &rive.KeyFrameColor{}
					f.Frame = kf.frame
					f.Value = kf.color
					f.InterpolationType = it
					f.InterpolatorId = interpID
					*objects = append(*objects, f)
				} else {
					f := &rive.KeyFrameDouble{}
					f.Frame = kf.frame
					f.Value = kf.value
					f.InterpolationType = it
					f.InterpolatorId = interpID
					*objects = append(*objects, f)
				}
			}
		}
	}

	// ── Animated draw order keyframes ────────────────────────────────────────
	// Emit KeyedObject → KeyedProperty(121) → KeyFrameId (hold) sequences.
	for _, src := range slotOrder {
		slot := slotMap[src]

		kobj := &rive.KeyedObject{}
		kobj.ObjectId = slot.rulesIdx
		*objects = append(*objects, kobj)

		kprop := &rive.KeyedProperty{}
		kprop.PropertyKey = uint64(PropDrawTargetId) // 121
		*objects = append(*objects, kprop)

		// Collect and sort keyframes for this source shape.
		type doKFEntry struct {
			frame uint64
			value uint64
		}
		var sorted []doKFEntry
		for _, kf := range a.drawOrderKFs {
			if kf.source != src {
				continue
			}
			v := ^uint64(0) // nil target → sentinel (no active rule)
			if kf.target != nil {
				k := doTargetKey{kf.target.shapeIdx, kf.placement}
				v = slot.dtMap[k]
			}
			sorted = append(sorted, doKFEntry{kf.frame, v})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].frame < sorted[j].frame })

		for _, kf := range sorted {
			f := &rive.KeyFrameId{}
			f.Frame = kf.frame
			f.Value = kf.value
			// InterpolationType=0 (hold) is the default and suppressed; InterpolatorId=^uint64(0) suppressed
			f.InterpolatorId = ^uint64(0)
			*objects = append(*objects, f)
		}
	}

	return nil
}
