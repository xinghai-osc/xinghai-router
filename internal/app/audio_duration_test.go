package app

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func durationTestWAV(format uint16, rate uint32, channels, bits uint16, samples int) []byte {
	align := channels * (bits / 8)
	dataSize := samples * int(align)
	body := make([]byte, 44+dataSize+dataSize%2)
	copy(body, "RIFF")
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(body)-8))
	copy(body[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(body[16:20], 16)
	binary.LittleEndian.PutUint16(body[20:22], format)
	binary.LittleEndian.PutUint16(body[22:24], channels)
	binary.LittleEndian.PutUint32(body[24:28], rate)
	binary.LittleEndian.PutUint32(body[28:32], rate*uint32(align))
	binary.LittleEndian.PutUint16(body[32:34], align)
	binary.LittleEndian.PutUint16(body[34:36], bits)
	copy(body[36:], "data")
	binary.LittleEndian.PutUint32(body[40:44], uint32(dataSize))
	return body
}

func durationTestMP3Frame(header uint32, size int) []byte {
	frame := make([]byte, size)
	binary.BigEndian.PutUint32(frame, header)
	return frame
}

func TestMediaAudioDurationWAV(t *testing.T) {
	for _, test := range []struct {
		name                   string
		format, channels, bits uint16
		rate                   uint32
		samples                int
	}{
		{"pcm8-mono-padded", 1, 1, 8, 8000, 8001},
		{"pcm16-stereo", 1, 2, 16, 44100, 88200},
		{"pcm24-stereo", 1, 2, 24, 48000, 24000},
		{"pcm32-mono", 1, 1, 32, 16000, 16000},
		{"float32-stereo", 3, 2, 32, 48000, 48000},
		{"float64-mono", 3, 1, 64, 16000, 8000},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := durationTestWAV(test.format, test.rate, test.channels, test.bits, test.samples)
			got, err := mediaAudioDuration(data)
			want := float64(test.samples) / float64(test.rate)
			if err != nil || math.Abs(got-want) > 1e-12 {
				t.Fatalf("duration=%v want=%v err=%v", got, want, err)
			}
		})
	}
}

func TestMediaAudioDurationWAVAncillaryChunks(t *testing.T) {
	data := durationTestWAV(1, 8000, 1, 16, 8000)
	chunk := []byte{'J', 'U', 'N', 'K', 3, 0, 0, 0, 1, 2, 3, 0}
	data = append(append(append([]byte{}, data[:12]...), chunk...), data[12:]...)
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	got, err := mediaAudioDuration(data)
	if err != nil || got != 1 {
		t.Fatalf("ancillary chunk duration=%v err=%v", got, err)
	}
}

func TestMediaAudioDurationRejectsInvalidWAV(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{"truncated", func(b []byte) []byte { return b[:len(b)-1] }},
		{"trailing-bytes", func(b []byte) []byte { return append(b, 0) }},
		{"forged-byte-rate", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[28:32], 1); return b }},
		{"forged-alignment", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[32:34], 1); return b }},
		{"zero-rate", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[24:28], 0); return b }},
		{"zero-channels", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[22:24], 0); return b }},
		{"compressed", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[20:22], 6); return b }},
		{"extensible", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[20:22], 0xfffe); return b }},
		{"invalid-bits", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[34:36], 12); return b }},
		{"invalid-float-bits", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[20:22], 3); return b }},
		{"partial-sample", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[40:44], uint32(len(b)-45)); return b }},
		{"oversized-data", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[40:44], 0xffffffff); return b }},
		{"missing-format", func(b []byte) []byte { copy(b[12:16], "JUNK"); return b }},
		{"missing-data", func(b []byte) []byte { copy(b[36:40], "JUNK"); return b }},
		{"duplicate-format", func(b []byte) []byte {
			b = append(b, b[12:36]...)
			binary.LittleEndian.PutUint32(b[4:8], uint32(len(b)-8))
			return b
		}},
		{"duplicate-data", func(b []byte) []byte {
			b = append(b, b[36:]...)
			binary.LittleEndian.PutUint32(b[4:8], uint32(len(b)-8))
			return b
		}},
		{"truncated-chunk-header", func(b []byte) []byte {
			b = append(b, 'J', 'U', 'N', 'K')
			binary.LittleEndian.PutUint32(b[4:8], uint32(len(b)-8))
			return b
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := mediaAudioDuration(test.edit(durationTestWAV(1, 8000, 1, 16, 8000))); err == nil {
				t.Fatalf("invalid WAV accepted: duration=%v", got)
			}
		})
	}
	if _, err := mediaAudioDuration(durationTestWAV(1, 8000, 1, 16, 0)); err == nil {
		t.Fatal("empty WAV accepted")
	}
}

func TestMediaAudioDurationMP3(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		want float64
	}{
		{"mpeg1", durationTestMP3Frame(0xfffb9000, 417), 1152.0 / 44100},
		{"mpeg2", durationTestMP3Frame(0xfff38000, 208), 576.0 / 22050},
		{"mpeg2.5", durationTestMP3Frame(0xffe38000, 417), 576.0 / 11025},
		{"mono-crc", durationTestMP3Frame(0xfffa90c0, 417), 1152.0 / 44100},
		{"padded", durationTestMP3Frame(0xfffb9200, 418), 1152.0 / 44100},
		{"vbr", append(durationTestMP3Frame(0xfffb9000, 417), durationTestMP3Frame(0xfffbb000, 626)...), 2304.0 / 44100},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := mediaAudioDuration(test.data)
			if err != nil || math.Abs(got-test.want) > 1e-12 {
				t.Fatalf("duration=%v want=%v err=%v", got, test.want, err)
			}
		})
	}
}

func TestMediaAudioDurationMP3Tags(t *testing.T) {
	frame := durationTestMP3Frame(0xfffb9000, 417)
	id3v1 := make([]byte, 128)
	copy(id3v1, "TAG")
	id3v2 := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 3, 1, 2, 3}
	footerTag := []byte{'I', 'D', '3', 4, 0, 0x10, 0, 0, 0, 3, 1, 2, 3, '3', 'D', 'I', 4, 0, 0x10, 0, 0, 0, 3}
	for _, prefix := range [][]byte{nil, id3v2, footerTag} {
		data := append(append(append([]byte{}, prefix...), frame...), id3v1...)
		got, err := mediaAudioDuration(data)
		if err != nil || math.Abs(got-1152.0/44100) > 1e-12 {
			t.Fatalf("tagged MP3 duration=%v err=%v", got, err)
		}
	}
	copy(frame[36:], "Xing")
	binary.BigEndian.PutUint32(frame[40:44], 1)
	binary.BigEndian.PutUint32(frame[44:48], 0xffffffff)
	got, err := mediaAudioDuration(frame)
	if err != nil || math.Abs(got-1152.0/44100) > 1e-12 {
		t.Fatalf("trusted forged Xing count: duration=%v err=%v", got, err)
	}
}

func TestMediaAudioDurationRejectsInvalidMP3(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"truncated-frame", durationTestMP3Frame(0xfffb9000, 416)},
		{"trailing-junk", append(durationTestMP3Frame(0xfffb9000, 417), 0)},
		{"free-bitrate", durationTestMP3Frame(0xfffb0000, 417)},
		{"reserved-bitrate", durationTestMP3Frame(0xfffbf000, 417)},
		{"reserved-sample-rate", durationTestMP3Frame(0xfffb9c00, 417)},
		{"reserved-version", durationTestMP3Frame(0xffeb9000, 417)},
		{"layer-two", durationTestMP3Frame(0xfffd9000, 417)},
		{"broken-sync", append(durationTestMP3Frame(0xfffb9000, 417), make([]byte, 417)...)},
		{"changed-rate", append(durationTestMP3Frame(0xfffb9000, 417), durationTestMP3Frame(0xfffb9400, 384)...)},
		{"changed-channels", append(durationTestMP3Frame(0xfffb9000, 417), durationTestMP3Frame(0xfffb90c0, 417)...)},
		{"truncated-id3", []byte("ID3")},
		{"id3-no-frames", []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}},
		{"id3-invalid-size", []byte{'I', 'D', '3', 3, 0, 0, 0x80, 0, 0, 0}},
		{"id3-invalid-version", []byte{'I', 'D', '3', 5, 0, 0, 0, 0, 0, 0}},
		{"id3-invalid-flags", []byte{'I', 'D', '3', 3, 0, 1, 0, 0, 0, 0}},
		{"id3-oversized", []byte{'I', 'D', '3', 3, 0, 0, 0x7f, 0x7f, 0x7f, 0x7f}},
		{"id3-missing-footer", []byte{'I', 'D', '3', 4, 0, 0x10, 0, 0, 0, 0}},
		{"id3-only-footer", []byte{'I', 'D', '3', 4, 0, 0x10, 0, 0, 0, 0, '3', 'D', 'I', 4, 0, 0x10, 0, 0, 0, 0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := mediaAudioDuration(test.data); err == nil {
				t.Fatalf("invalid MP3 accepted: duration=%v", got)
			}
		})
	}
}

func TestMediaAudioDurationRejectsUnknownFormats(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("RIFF"), []byte("OggS"), []byte("fLaC"), []byte("\x00\x00\x00\x18ftypM4A "), bytes.Repeat([]byte{0}, 100)} {
		if _, err := mediaAudioDuration(data); err == nil {
			t.Fatalf("unknown format accepted: %q", data)
		}
	}
}

func FuzzMediaAudioDuration(f *testing.F) {
	f.Add(durationTestWAV(1, 8000, 1, 16, 80))
	f.Add(durationTestMP3Frame(0xfffb9000, 417))
	f.Add([]byte("ID3"))
	f.Fuzz(func(t *testing.T, data []byte) {
		duration, err := mediaAudioDuration(data)
		if err == nil && (duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0)) {
			t.Fatalf("invalid successful duration %v", duration)
		}
	})
}
