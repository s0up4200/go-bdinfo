package codec

import "github.com/s0up4200/go-bdinfo/internal/stream"

func ScanMPEG2(v *stream.VideoStream, _ []byte) {
	if v.IsInitialized {
		return
	}
	v.IsVBR = true
	v.IsInitialized = true
}
