package codec

import "github.com/s0up4200/go-bdinfo/internal/stream"

func ScanPGS(g *stream.GraphicsStream, _ []byte) {
	if g.IsInitialized {
		return
	}
	g.IsInitialized = true
}
