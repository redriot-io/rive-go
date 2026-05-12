package builder

import "github.com/redriot-io/rive-go/rive"

// AudioAssetRef is a handle to an embedded audio asset added to an artboard.
// Create it via ArtboardBuilder.EmbedAudio.
type AudioAssetRef struct {
	name       string
	audioBytes []byte
	volume     float64
	idx        uint64 // 0-based index among AudioAssets in global stream, set on emit
}

// AudioOption configures optional fields on an AudioAssetRef.
type AudioOption func(*AudioAssetRef)

// WithVolume sets the playback volume (default 1.0, range 0–1).
func WithVolume(v float64) AudioOption { return func(a *AudioAssetRef) { a.volume = v } }

// AudioEventRef is a handle to an AudioEvent node inside an artboard.
// Create it via ArtboardBuilder.AudioEvent.
type AudioEventRef struct {
	name  string
	asset *AudioAssetRef
	idx   uint64 // artboard-relative index, set during emitObjects
}

// animIdx implements AnimTarget.
func (r *AudioEventRef) animIdx() uint64 { return r.idx }

// animColorIdx implements AnimTarget — audio events have no color property.
func (r *AudioEventRef) animColorIdx() (uint64, bool) { return 0, false }

// EmbedAudio registers an audio asset with raw audio bytes and returns an AudioAssetRef.
// The AudioAssetRef is passed to ArtboardBuilder.AudioEvent to place the event node.
// Supported opts: WithVolume.
func (ab *ArtboardBuilder) EmbedAudio(name string, audioBytes []byte, opts ...AudioOption) *AudioAssetRef {
	a := &AudioAssetRef{name: name, audioBytes: audioBytes, volume: 1.0}
	for _, opt := range opts {
		opt(a)
	}
	ab.audios = append(ab.audios, a)
	return a
}

// AudioEvent adds an AudioEvent node to the artboard referencing the given asset.
func (ab *ArtboardBuilder) AudioEvent(name string, asset *AudioAssetRef) *AudioEventRef {
	ref := &AudioEventRef{name: name, asset: asset}
	ab.children = append(ab.children, ref)
	return ref
}

func (r *AudioEventRef) emitObjects(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64) {
	r.idx = uint64(len(*objects)) - artboardOffset
	ae := &rive.AudioEvent{}
	ae.ParentId = parentIdx
	ae.Name = r.name
	ae.AssetId = r.asset.idx
	*objects = append(*objects, ae)
}
