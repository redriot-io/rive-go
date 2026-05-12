package builder

import "github.com/redriot-io/rive-go/rive"

// BoneRef is a handle to a Bone (typeKey=40) or RootBone (typeKey=41) node.
// Create via ArtboardBuilder.RootBone or BoneRef.AddBone.
type BoneRef struct {
	name     string
	isRoot   bool    // true → RootBone(41); false → Bone(40)
	length   float64
	rotation float64
	x, y     float64    // RootBone only
	parent   *BoneRef   // nil for root bones
	children []*BoneRef
	idx      uint64     // artboard-relative index, set during emit
}

// SkinBinding represents a Skin(43) + Tendons(44) attachment to a shape.
type SkinBinding struct {
	bones []*BoneRef
}

// BoneOption configures a bone's properties.
type BoneOption func(*BoneRef)

// WithLength sets the bone's length.
func WithLength(l float64) BoneOption { return func(b *BoneRef) { b.length = l } }

// WithRotation sets the bone's rotation in radians.
func WithRotation(r float64) BoneOption { return func(b *BoneRef) { b.rotation = r } }

// WithTranslation sets the root bone's x/y position (no effect on non-root bones).
func WithTranslation(x, y float64) BoneOption {
	return func(b *BoneRef) { b.x = x; b.y = y }
}

// RootBone adds a RootBone to the artboard and returns its ref.
// RootBones are emitted before regular children inside the artboard.
func (ab *ArtboardBuilder) RootBone(name string, opts ...BoneOption) *BoneRef {
	b := &BoneRef{name: name, isRoot: true}
	for _, opt := range opts {
		opt(b)
	}
	ab.boneTrees = append(ab.boneTrees, b)
	return b
}

// Bone adds a child Bone to the given parent (RootBone or Bone) and returns its ref.
func (ab *ArtboardBuilder) Bone(parent *BoneRef, name string, opts ...BoneOption) *BoneRef {
	b := &BoneRef{name: name, parent: parent}
	for _, opt := range opts {
		opt(b)
	}
	parent.children = append(parent.children, b)
	return b
}

// emitBoneTree recursively emits a bone and all its children, setting idx on each.
func emitBoneTree(b *BoneRef, objects *[]rive.Object, artboardOffset uint64) {
	b.idx = uint64(len(*objects)) - artboardOffset
	if b.isRoot {
		rb := &rive.RootBone{}
		rb.Name = b.name
		rb.Length = b.length
		rb.Rotation = b.rotation
		rb.X = b.x
		rb.Y = b.y
		// ParentId=0 (artboard default, not emitted)
		*objects = append(*objects, rb)
	} else {
		bone := &rive.Bone{}
		bone.Name = b.name
		bone.Length = b.length
		bone.Rotation = b.rotation
		if b.parent != nil {
			bone.ParentId = b.parent.idx
		}
		*objects = append(*objects, bone)
	}
	for _, child := range b.children {
		emitBoneTree(child, objects, artboardOffset)
	}
}

// BindSkin attaches a Skin + Tendons to this shape, binding it to the given bones.
// Call after declaring all bones so their indices are set during emission.
func (s *ShapeRef) BindSkin(bones ...*BoneRef) *ShapeRef {
	s.skins = append(s.skins, &SkinBinding{bones: bones})
	return s
}

// emitSkins is called from ShapeRef.emitObjects after all shape children are emitted.
func (s *ShapeRef) emitSkins(objects *[]rive.Object, artboardOffset uint64) {
	for _, sb := range s.skins {
		skinIdx := uint64(len(*objects)) - artboardOffset
		skin := &rive.Skin{}
		skin.ParentId = s.shapeIdx // child of this shape
		*objects = append(*objects, skin)

		for _, bone := range sb.bones {
			tendon := &rive.Tendon{}
			tendon.ParentId = skinIdx // child of Skin
			tendon.BoneId = bone.idx  // references the bone by artboard-relative index
			*objects = append(*objects, tendon)
		}
	}
}
