package codec

import (
	"encoding/binary"

	"github.com/autobrr/go-bdinfo/internal/stream"
)

func ScanDTSHD(a *stream.AudioStream, data []byte, fallbackBitrate int64) {
	if a.IsInitialized && (a.StreamType == stream.StreamTypeDTSHDSecondaryAudio || (a.CoreStream != nil && a.CoreStream.IsInitialized)) {
		return
	}
	syncOffset := -1
	for i := 0; i+4 <= len(data); i++ {
		if binary.BigEndian.Uint32(data[i:i+4]) == 0x64582025 {
			syncOffset = i
			break
		}
	}
	if syncOffset == -1 {
		// fall back to core
		if a.CoreStream == nil {
			a.CoreStream = &stream.AudioStream{Stream: stream.Stream{StreamType: stream.StreamTypeDTSAudio}}
		}
		if !a.CoreStream.IsInitialized {
			ScanDTS(a.CoreStream, data, fallbackBitrate)
		}
		return
	}

	a.HasExtensions = true
	a.IsVBR = true
	// keep core attributes if present
	if a.CoreStream != nil {
		if a.SampleRate == 0 {
			a.SampleRate = a.CoreStream.SampleRate
		}
		if a.ChannelCount == 0 {
			a.ChannelCount = a.CoreStream.ChannelCount
		}
		if a.LFE == 0 {
			a.LFE = a.CoreStream.LFE
		}
	}
	a.IsInitialized = true
}
