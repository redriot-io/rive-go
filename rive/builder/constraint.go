package builder

import "github.com/redriot-io/rive-go/rive"

// constraintEmitter is the internal interface for constraint objects.
type constraintEmitter interface {
	emitConstraint(objects *[]rive.Object, artboardOffset uint64)
}

// constraintConfig holds all possible constraint parameters.
// Defaults: strength=1.0, distance=100.0, target=nil (→ sentinel ^uint64(0)).
type constraintConfig struct {
	name            string
	constrained     *BoneRef
	target          *BoneRef
	strength        float64
	invertDirection bool
	parentBoneCount uint64
	distance        float64
	modeValue       uint64
	originX, originY       float64
	sourceSpace, destSpace uint64
}

// ConstraintOption is a functional option for constraint constructors.
type ConstraintOption func(*constraintConfig)

func newConstraintConfig(name string, constrained, target *BoneRef, opts []ConstraintOption) constraintConfig {
	cfg := constraintConfig{
		name:        name,
		constrained: constrained,
		target:      target,
		strength:    1.0,
		distance:    100.0,
	}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// WithConstraintStrength sets the constraint strength (0–1, default 1.0).
func WithConstraintStrength(v float64) ConstraintOption {
	return func(c *constraintConfig) { c.strength = v }
}

// WithInvertDirection sets IK invert-direction flag.
func WithInvertDirection(v bool) ConstraintOption {
	return func(c *constraintConfig) { c.invertDirection = v }
}

// WithChainLength sets the IK parentBoneCount (chain length).
func WithChainLength(n int) ConstraintOption {
	return func(c *constraintConfig) { c.parentBoneCount = uint64(n) }
}

// WithConstraintDistance sets the DistanceConstraint distance (default 100).
func WithConstraintDistance(d float64) ConstraintOption {
	return func(c *constraintConfig) { c.distance = d }
}

// WithConstraintMode sets the DistanceConstraint modeValue.
func WithConstraintMode(m uint64) ConstraintOption {
	return func(c *constraintConfig) { c.modeValue = m }
}

// WithConstraintOrigin sets TransformConstraint originX/Y.
func WithConstraintOrigin(x, y float64) ConstraintOption {
	return func(c *constraintConfig) { c.originX = x; c.originY = y }
}

// targetIdx returns the artboard-relative target index, or the sentinel if nil.
func (cfg *constraintConfig) targetIdx() uint64 {
	if cfg.target != nil {
		return cfg.target.idx
	}
	return ^uint64(0)
}

// ── IKConstraintRef ───────────────────────────────────────────────────────────

// IKConstraintRef is a handle to an IK constraint added to an artboard.
type IKConstraintRef struct{ cfg constraintConfig }

func (r *IKConstraintRef) emitConstraint(objects *[]rive.Object, artboardOffset uint64) {
	ik := &rive.IKConstraint{}
	ik.Name = r.cfg.name
	ik.ParentId = r.cfg.constrained.idx
	ik.TargetId = r.cfg.targetIdx()
	ik.Strength = r.cfg.strength
	ik.InvertDirection = r.cfg.invertDirection
	ik.ParentBoneCount = r.cfg.parentBoneCount
	*objects = append(*objects, ik)
}

// ── DistanceConstraintRef ─────────────────────────────────────────────────────

// DistanceConstraintRef is a handle to a distance constraint added to an artboard.
type DistanceConstraintRef struct{ cfg constraintConfig }

func (r *DistanceConstraintRef) emitConstraint(objects *[]rive.Object, artboardOffset uint64) {
	dc := &rive.DistanceConstraint{}
	dc.Name = r.cfg.name
	dc.ParentId = r.cfg.constrained.idx
	dc.TargetId = r.cfg.targetIdx()
	dc.Strength = r.cfg.strength
	dc.Distance = r.cfg.distance
	dc.ModeValue = r.cfg.modeValue
	*objects = append(*objects, dc)
}

// ── TransformConstraintRef ────────────────────────────────────────────────────

// TransformConstraintRef is a handle to a transform constraint added to an artboard.
type TransformConstraintRef struct{ cfg constraintConfig }

func (r *TransformConstraintRef) emitConstraint(objects *[]rive.Object, artboardOffset uint64) {
	tc := &rive.TransformConstraint{}
	tc.Name = r.cfg.name
	tc.ParentId = r.cfg.constrained.idx
	tc.TargetId = r.cfg.targetIdx()
	tc.Strength = r.cfg.strength
	tc.OriginX = r.cfg.originX
	tc.OriginY = r.cfg.originY
	tc.SourceSpaceValue = r.cfg.sourceSpace
	tc.DestSpaceValue = r.cfg.destSpace
	*objects = append(*objects, tc)
}
