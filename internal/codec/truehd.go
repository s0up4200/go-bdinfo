package codec

import (
	"encoding/binary"

	"github.com/autobrr/go-bdinfo/internal/buffer"
	"github.com/autobrr/go-bdinfo/internal/stream"
)

func ScanTrueHD(a *stream.AudioStream, data []byte) {
	if a.IsInitialized && (a.CoreStream == nil || a.CoreStream.IsInitialized) {
		return
	}
	syncOffset := -1
	for i := 0; i+4 <= len(data); i++ {
		if binary.BigEndian.Uint32(data[i:i+4]) == 0xF8726FBA {
			syncOffset = i
			break
		}
	}
	if syncOffset == -1 {
		if a.CoreStream == nil {
			a.CoreStream = &stream.AudioStream{Stream: stream.Stream{StreamType: stream.StreamTypeAC3Audio}}
		}
		if !a.CoreStream.IsInitialized {
			ScanAC3(a.CoreStream, data)
		}
		return
	}

	br := buffer.NewBitReader(data[syncOffset+4:])
	ratebits, _ := br.ReadBits(4)
	if ratebits != 0xF {
		base := 48000
		if (ratebits & 8) > 0 {
			base = 44100
		}
		a.SampleRate = base << (ratebits & 7)
	}
	_, _ = br.ReadBits(15)

	a.ChannelCount = 0
	a.LFE = 0
	flags := []int{1, 1, 2, 2, 1, 1, 2, 2, 2, 2, 1, 1, 2}
	for i, add := range flags {
		bit, _ := br.ReadBit()
		if bit == 1 {
			if i == 0 || i == 10 {
				a.LFE += 1
			} else {
				a.ChannelCount += add
			}
		}
	}

	_, _ = br.ReadBits(49)

	peakBitrate, _ := br.ReadBits(15)
	peakBitrate = (peakBitrate * uint64(a.SampleRate)) >> 4
	if a.SampleRate > 0 {
		peakBitdepth := float64(peakBitrate) / float64(a.ChannelCount+a.LFE) / float64(a.SampleRate)
		if peakBitdepth > 14 {
			a.BitDepth = 24
		} else {
			a.BitDepth = 16
		}
	}

	_, _ = br.ReadBits(79)
	hasExtensionsBit, _ := br.ReadBit()
	numExtensions, _ := br.ReadBits(4)
	numExtensions = numExtensions*2 + 1
	hasContent, _ := br.ReadBits(4)
	if hasExtensionsBit == 1 {
		for idx := uint64(0); idx < numExtensions; idx++ {
			b, _ := br.ReadBits(8)
			if b != 0 {
				hasContent = 1
			}
		}
		if hasContent != 0 {
			a.HasExtensions = true
		}
	}

	a.IsVBR = true
	if a.CoreStream == nil || a.CoreStream.IsInitialized {
		a.IsInitialized = true
	}
}
