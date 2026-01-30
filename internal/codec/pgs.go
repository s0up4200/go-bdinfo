package codec

import "github.com/autobrr/go-bdinfo/internal/stream"

func ScanPGS(g *stream.GraphicsStream, _ []byte) {
	if g.IsInitialized {
		return
	}
	g.IsInitialized = true
}
