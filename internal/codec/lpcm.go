package codec

import "github.com/s0up4200/go-bdinfo/internal/stream"

func ScanLPCM(a *stream.AudioStream, _ []byte) {
	if a.IsInitialized {
		return
	}
	a.IsVBR = false
	a.IsInitialized = true
}
