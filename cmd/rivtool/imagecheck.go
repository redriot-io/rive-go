package main

import (
	"fmt"

	"github.com/redriot-io/rive-go/rive"
)

var (
	pngMagic  = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	jpegMagic = []byte{0xff, 0xd8, 0xff}
)

func hasMagic(b []byte, magic []byte) bool {
	if len(b) < len(magic) {
		return false
	}
	for i, v := range magic {
		if b[i] != v {
			return false
		}
	}
	return true
}

// verifyImages checks every FileAssetContents that follows an ImageAsset in
// the object stream. It validates the magic bytes (PNG or JPEG) and warns
// if the payload exceeds 1 MiB.
func verifyImages(f *rive.File) (passes, errs []string) {
	objs := f.Objects
	for i := 0; i < len(objs); i++ {
		if objs[i].TypeKey() != 105 { // ImageAsset
			continue
		}
		name := ""
		for _, p := range objs[i].Properties() {
			if p.Key == 203 {
				name, _ = p.Value.(string)
			}
		}
		if name == "" {
			name = fmt.Sprintf("ImageAsset[%d]", i)
		}

		if i+1 >= len(objs) || objs[i+1].TypeKey() != 106 { // FileAssetContents
			errs = append(errs, fmt.Sprintf("image %q: no FileAssetContents follows ImageAsset", name))
			continue
		}
		i++ // advance past FileAssetContents

		var imgBytes []byte
		for _, p := range objs[i].Properties() {
			if p.Key == 212 {
				imgBytes, _ = p.Value.([]byte)
			}
		}
		if len(imgBytes) == 0 {
			errs = append(errs, fmt.Sprintf("image %q: FileAssetContents has no bytes", name))
			continue
		}

		switch {
		case hasMagic(imgBytes, pngMagic):
			passes = append(passes, fmt.Sprintf("image %q: valid PNG (%d bytes)", name, len(imgBytes)))
		case hasMagic(imgBytes, jpegMagic):
			passes = append(passes, fmt.Sprintf("image %q: valid JPEG (%d bytes)", name, len(imgBytes)))
		default:
			errs = append(errs, fmt.Sprintf("image %q: embedded image is not valid PNG or JPEG (first bytes: %x)", name, imgBytes[:min8(len(imgBytes))]))
		}

		if len(imgBytes) > 1<<20 {
			passes = append(passes, fmt.Sprintf("⚠ image %q: large payload (%d bytes > 1 MiB)", name, len(imgBytes)))
		}
	}
	return
}

func min8(n int) int {
	if n < 8 {
		return n
	}
	return 8
}
