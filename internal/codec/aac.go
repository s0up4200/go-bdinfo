package codec

import "github.com/s0up4200/go-bdinfo/internal/stream"

var aacSampleRates = []int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}

func ScanAAC(a *stream.AudioStream, data []byte) {
	if a.IsInitialized {
		return
	}
	if len(data) < 7 {
		return
	}
	if data[0] != 0xFF || (data[1]&0xF0) != 0xF0 {
		return
	}
	sampleRateIdx := (data[2] >> 2) & 0x0F
	channelCfg := ((data[2] & 0x01) << 2) | (data[3] >> 6)
	if int(sampleRateIdx) < len(aacSampleRates) {
		a.SampleRate = aacSampleRates[sampleRateIdx]
	}
	if channelCfg > 0 {
		a.ChannelCount = int(channelCfg)
	}
	a.IsInitialized = true
}
