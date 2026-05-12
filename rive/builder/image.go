package builder

import "github.com/redriot-io/rive-go/rive"

// ImageRef is a handle to an embedded image asset added to an artboard.
// Create it via ArtboardBuilder.EmbedImage.
type ImageRef struct {
	name     string
	pngBytes []byte
	idx      uint64 // 0-based index among ImageAssets in global stream, set on emit
}

// ImageNodeRef is a handle to an Image drawable node inside an artboard.
// Create it via ArtboardBuilder.Image.
type ImageNodeRef struct {
	asset          *ImageRef
	x, y           float64
	originX, originY float64
	idx            uint64 // artboard-relative index, set during emitObjects
}

// animIdx implements AnimTarget.
func (n *ImageNodeRef) animIdx() uint64 { return n.idx }

// animColorIdx implements AnimTarget — image nodes have no color property.
func (n *ImageNodeRef) animColorIdx() (uint64, bool) { return 0, false }

// EmbedImage registers an image asset with raw PNG bytes and returns an ImageRef.
// The ImageRef is passed to ArtboardBuilder.Image to place the image drawable.
func (ab *ArtboardBuilder) EmbedImage(name string, pngBytes []byte) *ImageRef {
	img := &ImageRef{name: name, pngBytes: pngBytes}
	ab.images = append(ab.images, img)
	return img
}

// Image adds an Image drawable node to the artboard referencing the given asset.
func (ab *ArtboardBuilder) Image(asset *ImageRef) *ImageNodeRef {
	node := &ImageNodeRef{asset: asset, originX: 0.5, originY: 0.5}
	ab.children = append(ab.children, node)
	return node
}

// Position sets the x/y position of the image node.
func (n *ImageNodeRef) Position(x, y float64) *ImageNodeRef {
	n.x = x
	n.y = y
	return n
}

// Origin sets the origin (pivot) of the image node. Default is 0.5, 0.5 (center).
func (n *ImageNodeRef) Origin(x, y float64) *ImageNodeRef {
	n.originX = x
	n.originY = y
	return n
}

func (n *ImageNodeRef) emitObjects(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64) {
	n.idx = uint64(len(*objects)) - artboardOffset
	img := &rive.Image{}
	img.ParentId = parentIdx
	img.X = n.x
	img.Y = n.y
	img.AssetId = n.asset.idx
	img.OriginX = n.originX
	img.OriginY = n.originY
	*objects = append(*objects, img)
}
