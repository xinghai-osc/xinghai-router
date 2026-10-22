package app

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func mediaAudioDuration(data []byte) (float64, error) {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return mediaWAVDuration(data)
	}
	if len(data) >= 3 && string(data[:3]) == "ID3" || len(data) >= 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0 {
		return mediaMP3Duration(data)
	}
	return 0, fmt.Errorf("audio file must be PCM/IEEE float WAV or MPEG Layer III MP3")
}

func mediaWAVDuration(data []byte) (float64, error) {
	if uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
		return 0, fmt.Errorf("invalid WAV RIFF size")
	}
	var sampleRate, byteRate uint32
	var blockAlign uint16
	var dataSize uint64
	var haveFormat, haveData bool
	for offset := 12; offset < len(data); {
		if len(data)-offset < 8 {
			return 0, fmt.Errorf("truncated WAV chunk header")
		}
		name := string(data[offset : offset+4])
		size := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		padded := size + size%2
		if padded > uint64(len(data)-offset) {
			return 0, fmt.Errorf("truncated WAV chunk")
		}
		chunk := data[offset : offset+int(size)]
		switch name {
		case "fmt ":
			if haveFormat || len(chunk) < 16 || len(chunk) == 17 {
				return 0, fmt.Errorf("invalid WAV format chunk")
			}
			if len(chunk) >= 18 && int(binary.LittleEndian.Uint16(chunk[16:18])) != len(chunk)-18 {
				return 0, fmt.Errorf("invalid WAV format extension")
			}
			format := binary.LittleEndian.Uint16(chunk[:2])
			channels := binary.LittleEndian.Uint16(chunk[2:4])
			sampleRate = binary.LittleEndian.Uint32(chunk[4:8])
			byteRate = binary.LittleEndian.Uint32(chunk[8:12])
			blockAlign = binary.LittleEndian.Uint16(chunk[12:14])
			bits := binary.LittleEndian.Uint16(chunk[14:16])
			if format == 1 {
				if bits != 8 && bits != 16 && bits != 24 && bits != 32 {
					return 0, fmt.Errorf("unsupported WAV PCM bit depth")
				}
			} else if format == 3 {
				if bits != 32 && bits != 64 {
					return 0, fmt.Errorf("unsupported WAV float bit depth")
				}
			} else {
				return 0, fmt.Errorf("compressed or extensible WAV is not supported")
			}
			if channels == 0 || sampleRate == 0 || blockAlign == 0 || uint64(blockAlign) != uint64(channels)*uint64(bits/8) || uint64(byteRate) != uint64(sampleRate)*uint64(blockAlign) {
				return 0, fmt.Errorf("inconsistent WAV sample rate or alignment")
			}
			haveFormat = true
		case "data":
			if !haveFormat || haveData || size == 0 || size%uint64(blockAlign) != 0 {
				return 0, fmt.Errorf("invalid WAV audio data")
			}
			dataSize = size
			haveData = true
		}
		offset += int(padded)
	}
	if !haveFormat || !haveData {
		return 0, fmt.Errorf("WAV format and audio data are required")
	}
	return float64(dataSize/uint64(blockAlign)) / float64(sampleRate), nil
}

func mediaMP3Duration(data []byte) (float64, error) {
	offset := 0
	if len(data) >= 3 && string(data[:3]) == "ID3" {
		if len(data) < 10 || data[3] < 2 || data[3] > 4 || data[4] == 0xff {
			return 0, fmt.Errorf("invalid MP3 ID3v2 header")
		}
		allowedFlags := byte(0xc0)
		if data[3] == 3 {
			allowedFlags = 0xe0
		} else if data[3] == 4 {
			allowedFlags = 0xf0
		}
		if data[5]&^allowedFlags != 0 {
			return 0, fmt.Errorf("invalid MP3 ID3v2 flags")
		}
		size := 0
		for _, value := range data[6:10] {
			if value&0x80 != 0 {
				return 0, fmt.Errorf("invalid MP3 ID3v2 size")
			}
			size = size<<7 | int(value)
		}
		offset = 10 + size
		if offset > len(data) {
			return 0, fmt.Errorf("truncated MP3 ID3v2 tag")
		}
		if data[3] == 4 && data[5]&0x10 != 0 {
			if len(data)-offset < 10 || string(data[offset:offset+3]) != "3DI" || !bytes.Equal(data[offset+3:offset+10], data[3:10]) {
				return 0, fmt.Errorf("invalid MP3 ID3v2 footer")
			}
			offset += 10
		}
	}
	mpeg1Rates := [...]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
	mpeg2Rates := [...]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
	sampleRates := [...]int{44100, 48000, 32000}
	var samples int64
	var rate, version, channels int
	for offset < len(data) {
		if len(data)-offset == 128 && string(data[offset:offset+3]) == "TAG" {
			offset += 128
			break
		}
		if len(data)-offset < 4 {
			return 0, fmt.Errorf("truncated MP3 frame header")
		}
		header := binary.BigEndian.Uint32(data[offset : offset+4])
		frameVersion := int(header >> 19 & 3)
		layer := header >> 17 & 3
		bitrateIndex := int(header >> 12 & 15)
		sampleRateIndex := int(header >> 10 & 3)
		if header>>21 != 0x7ff || frameVersion == 1 || layer != 1 || bitrateIndex == 0 || bitrateIndex == 15 || sampleRateIndex == 3 || header&3 == 2 {
			return 0, fmt.Errorf("invalid or unsupported MP3 frame")
		}
		frameRate := sampleRates[sampleRateIndex]
		frameSamples, coefficient, bitrate := 1152, 144, mpeg1Rates[bitrateIndex]
		frameChannels := 2
		if header>>6&3 == 3 {
			frameChannels = 1
		}
		sideInfo := 32
		if frameChannels == 1 {
			sideInfo = 17
		}
		if frameVersion != 3 {
			frameRate /= 2
			if frameVersion == 0 {
				frameRate /= 2
			}
			frameSamples, coefficient, bitrate = 576, 72, mpeg2Rates[bitrateIndex]
			sideInfo = 17
			if frameChannels == 1 {
				sideInfo = 9
			}
		}
		frameSize := coefficient*bitrate*1000/frameRate + int(header>>9&1)
		minimumSize := 4 + sideInfo
		if header>>16&1 == 0 {
			minimumSize += 2
		}
		if frameSize < minimumSize || frameSize > len(data)-offset {
			return 0, fmt.Errorf("truncated MP3 frame")
		}
		if samples != 0 && (frameRate != rate || frameVersion != version || frameChannels != channels) {
			return 0, fmt.Errorf("MP3 stream format changes are not supported")
		}
		rate, version, channels = frameRate, frameVersion, frameChannels
		samples += int64(frameSamples)
		offset += frameSize
	}
	if samples == 0 {
		return 0, fmt.Errorf("MP3 audio frames are required")
	}
	return float64(samples) / float64(rate), nil
}
