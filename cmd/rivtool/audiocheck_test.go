package main

import (
	"strings"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// minimalWAVTest is a 44-byte silent WAV for testing.
var minimalWAVTest = []byte{
	0x52, 0x49, 0x46, 0x46, 0x24, 0x00, 0x00, 0x00,
	0x57, 0x41, 0x56, 0x45, 0x66, 0x6d, 0x74, 0x20,
	0x10, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00,
	0x40, 0x1f, 0x00, 0x00, 0x40, 0x1f, 0x00, 0x00,
	0x01, 0x00, 0x08, 0x00, 0x64, 0x61, 0x74, 0x61,
	0x00, 0x00, 0x00, 0x00,
}

func buildAudioTestRiv(t *testing.T, name string, audioBytes []byte) *rive.File {
	t.Helper()
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)
	asset := ab.EmbedAudio(name, audioBytes)
	ab.AudioEvent("sound", asset)
	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return f
}

func TestAudioCheck_ValidWAV(t *testing.T) {
	f := buildAudioTestRiv(t, "step", minimalWAVTest)
	passes, errs := verifyAudio(f)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
	if len(passes) == 0 {
		t.Error("expected at least one pass message")
	}
}

func TestAudioCheck_ValidMP3_ID3(t *testing.T) {
	mp3 := make([]byte, 64)
	mp3[0], mp3[1], mp3[2] = 0x49, 0x44, 0x33 // "ID3"
	f := buildAudioTestRiv(t, "music", mp3)
	passes, errs := verifyAudio(f)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
	if len(passes) == 0 {
		t.Error("expected at least one pass message")
	}
}

func TestAudioCheck_InvalidMagic(t *testing.T) {
	bad := make([]byte, 16)
	f := buildAudioTestRiv(t, "bad", bad)
	_, errs := verifyAudio(f)
	if len(errs) == 0 {
		t.Error("expected error for invalid audio magic bytes")
	}
}

func TestAudioCheck_OversizedWarning(t *testing.T) {
	const size = 6 * 1024 * 1024
	big := make([]byte, size)
	big[0], big[1], big[2], big[3] = 0x52, 0x49, 0x46, 0x46 // RIFF magic
	f := buildAudioTestRiv(t, "bigaudio", big)
	passes, errs := verifyAudio(f)
	if len(errs) != 0 {
		t.Errorf("expected no hard errors, got: %v", errs)
	}
	warnFound := false
	for _, p := range passes {
		if strings.Contains(p, "5 MiB") {
			warnFound = true
		}
	}
	if !warnFound {
		t.Errorf("expected oversize warning, got passes: %v", passes)
	}
}
