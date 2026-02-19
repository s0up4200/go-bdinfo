package bdrom

import (
	"testing"

	"github.com/autobrr/go-bdinfo/internal/stream"
)

func TestCompareGraphicsStreams_EnglishPIDDescendingWithinLanguage(t *testing.T) {
	a := stream.NewGraphicsStream()
	a.StreamType = stream.StreamTypePresentationGraphics
	a.PID = 2000
	a.SetLanguageCode("eng")

	b := stream.NewGraphicsStream()
	b.StreamType = stream.StreamTypePresentationGraphics
	b.PID = 3000
	b.SetLanguageCode("eng")

	if got := compareGraphicsStreams(a, b); got <= 0 {
		t.Fatalf("expected lower PID english stream after higher PID, got compare=%d", got)
	}
	if got := compareGraphicsStreams(b, a); got >= 0 {
		t.Fatalf("expected higher PID english stream first, got compare=%d", got)
	}
}
