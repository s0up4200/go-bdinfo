package codec

import (
	"github.com/autobrr/go-bdinfo/internal/buffer"
	"github.com/autobrr/go-bdinfo/internal/stream"
)

var ac3BitrateKbps = []int{32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 448, 512, 576, 640}
var ac3Channels = []int{2, 1, 2, 3, 3, 4, 4, 5}

func ScanAC3(a *stream.AudioStream, data []byte) {
	if a.IsInitialized {
		return
	}
	if len(data) < 7 {
		return
	}
	if data[0] != 0x0b || data[1] != 0x77 {
		return
	}
	br := buffer.NewBitReader(data)
	_, _ = br.ReadBits(16) // sync
	_, _ = br.ReadBits(16) // crc1
	fscod, ok := br.ReadBits(2)
	if !ok {
		return
	}
	frmsizecod, _ := br.ReadBits(6)
	bsid, _ := br.ReadBits(5)
	_, _ = br.ReadBits(3) // bsmod
	acmod, _ := br.ReadBits(3)
	if acmod == 2 { // stereo
		_, _ = br.ReadBits(2) // dsurmod
	}
	lfeon, _ := br.ReadBits(1)

	if bsid <= 10 {
		sampleRates := []int{48000, 44100, 32000}
		if fscod < 3 {
			a.SampleRate = sampleRates[fscod]
		}
		if int(frmsizecod/2) < len(ac3BitrateKbps) {
			a.BitRate = int64(ac3BitrateKbps[frmsizecod/2] * 1000)
		}
		if int(acmod) < len(ac3Channels) {
			a.ChannelCount = ac3Channels[acmod]
		}
		if lfeon > 0 {
			a.LFE = 1
		}
		a.IsInitialized = true
		return
	}

	// E-AC3 minimal parse
	br = buffer.NewBitReader(data)
	_, _ = br.ReadBits(16)
	_, _ = br.ReadBits(16)
	_, _ = br.ReadBits(2)  // strmtyp
	_, _ = br.ReadBits(3)  // substreamid
	_, _ = br.ReadBits(11) // frmsiz
	fscod, _ = br.ReadBits(2)
	if fscod == 3 {
		fscod2, _ := br.ReadBits(2)
		sampleRates2 := []int{24000, 22050, 16000}
		if fscod2 < 3 {
			a.SampleRate = sampleRates2[fscod2]
		}
	} else {
		sampleRates := []int{48000, 44100, 32000}
		if fscod < 3 {
			a.SampleRate = sampleRates[fscod]
		}
		_, _ = br.ReadBits(2) // numblkscod
	}
	acmod, _ = br.ReadBits(3)
	lfeon, _ = br.ReadBits(1)
	if int(acmod) < len(ac3Channels) {
		a.ChannelCount = ac3Channels[acmod]
	}
	if lfeon > 0 {
		a.LFE = 1
	}
	a.HasExtensions = true
	a.IsInitialized = true
}
