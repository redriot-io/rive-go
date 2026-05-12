package main

import (
	"fmt"

	"github.com/redriot-io/rive-go/rive"
)

var (
	mp3Magic1 = []byte{0xff, 0xfb}             // MP3 frame sync (no ID3 tag)
	mp3Magic2 = []byte{0x49, 0x44, 0x33}       // "ID3" tag prefix
	wavMagic  = []byte{0x52, 0x49, 0x46, 0x46} // "RIFF"
	oggMagic  = []byte{0x4f, 0x67, 0x67, 0x53} // "OggS"
)

func audioFormatName(b []byte) string {
	switch {
	case hasMagic(b, wavMagic):
		return "WAV"
	case hasMagic(b, oggMagic):
		return "OGG"
	case hasMagic(b, mp3Magic2):
		return "MP3 (ID3)"
	case hasMagic(b, mp3Magic1):
		return "MP3"
	default:
		return ""
	}
}

// verifyAudio checks every FileAssetContents that follows an AudioAsset (typeKey=406)
// in the object stream. It validates audio magic bytes (MP3/WAV/OGG) and warns if
// the payload exceeds 5 MiB.
func verifyAudio(f *rive.File) (passes, errs []string) {
	objs := f.Objects
	for i := 0; i < len(objs); i++ {
		if objs[i].TypeKey() != 406 { // AudioAsset
			continue
		}
		name := ""
		for _, p := range objs[i].Properties() {
			if p.Key == 203 {
				name, _ = p.Value.(string)
			}
		}
		if name == "" {
			name = fmt.Sprintf("AudioAsset[%d]", i)
		}

		if i+1 >= len(objs) || objs[i+1].TypeKey() != 106 { // FileAssetContents
			errs = append(errs, fmt.Sprintf("audio %q: no FileAssetContents follows AudioAsset", name))
			continue
		}
		i++ // advance past FileAssetContents

		var audioBytes []byte
		for _, p := range objs[i].Properties() {
			if p.Key == 212 {
				audioBytes, _ = p.Value.([]byte)
			}
		}
		if len(audioBytes) == 0 {
			errs = append(errs, fmt.Sprintf("audio %q: FileAssetContents has no bytes", name))
			continue
		}

		fmtName := audioFormatName(audioBytes)
		if fmtName == "" {
			errs = append(errs, fmt.Sprintf("audio %q: embedded audio is not valid MP3, WAV, or OGG (first bytes: %x)", name, audioBytes[:min8(len(audioBytes))]))
		} else {
			passes = append(passes, fmt.Sprintf("audio %q: valid %s (%d bytes)", name, fmtName, len(audioBytes)))
		}

		const warnSize = 5 << 20 // 5 MiB
		if len(audioBytes) > warnSize {
			passes = append(passes, fmt.Sprintf("⚠ audio %q: large payload (%d bytes > 5 MiB)", name, len(audioBytes)))
		}
	}
	return
}
