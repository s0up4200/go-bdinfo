package bdrom

import (
	"os"
	"testing"

	"github.com/autobrr/go-bdinfo/internal/settings"
)

func BenchmarkScan(b *testing.B) {
	path := os.Getenv("BDINFO_BENCH_PATH")
	if path == "" {
		b.Skip("BDINFO_BENCH_PATH not set")
	}
	b.ReportAllocs()
	for b.Loop() {
		rom, err := New(path, settings.Default("."))
		if err != nil {
			b.Fatal(err)
		}
		_ = rom.Scan()
		rom.Close()
	}
}
