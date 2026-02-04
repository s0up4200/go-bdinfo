package bdrom

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/autobrr/go-bdinfo/internal/settings"
)

// TestPlaylistFile_TotalLength tests duration calculation
func TestPlaylistFile_TotalLength(t *testing.T) {
	tests := []struct {
		name     string
		clips    []*StreamClip
		expected float64
	}{
		{
			name: "single clip",
			clips: []*StreamClip{
				{Length: 100.0, AngleIndex: 0},
			},
			expected: 100.0,
		},
		{
			name: "multiple clips",
			clips: []*StreamClip{
				{Length: 100.0, AngleIndex: 0},
				{Length: 200.0, AngleIndex: 0},
				{Length: 50.0, AngleIndex: 0},
			},
			expected: 350.0,
		},
		{
			name: "clips with angles",
			clips: []*StreamClip{
				{Length: 100.0, AngleIndex: 0},
				{Length: 50.0, AngleIndex: 1}, // Angle, shouldn't count
				{Length: 200.0, AngleIndex: 0},
			},
			expected: 300.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			playlist := &PlaylistFile{
				StreamClips: tt.clips,
			}
			got := playlist.TotalLength()
			if got != tt.expected {
				t.Errorf("TotalLength() = %.1f, want %.1f", got, tt.expected)
			}
		})
	}
}

// TestPlaylistFile_TotalAngleLength tests total length including angles
func TestPlaylistFile_TotalAngleLength(t *testing.T) {
	playlist := &PlaylistFile{
		StreamClips: []*StreamClip{
			{Length: 100.0, AngleIndex: 0},
			{Length: 50.0, AngleIndex: 1},
			{Length: 200.0, AngleIndex: 0},
		},
	}

	expected := 350.0 // All clips including angles
	got := playlist.TotalAngleLength()
	if got != expected {
		t.Errorf("TotalAngleLength() = %.1f, want %.1f", got, expected)
	}
}

// TestPlaylistFile_FileSize tests total file size calculation
func TestPlaylistFile_FileSize(t *testing.T) {
	tests := []struct {
		name     string
		clips    []*StreamClip
		expected uint64
	}{
		{
			name: "single file",
			clips: []*StreamClip{
				{FileSize: 1000000},
			},
			expected: 1000000,
		},
		{
			name: "multiple files",
			clips: []*StreamClip{
				{FileSize: 1000000},
				{FileSize: 2000000},
				{FileSize: 500000},
			},
			expected: 3500000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			playlist := &PlaylistFile{
				StreamClips: tt.clips,
			}
			got := playlist.FileSize()
			if got != tt.expected {
				t.Errorf("FileSize() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestPlaylistFile_IsValid tests playlist validation
func TestPlaylistFile_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		playlist *PlaylistFile
		want     bool
	}{
		{
			name: "valid long playlist",
			playlist: &PlaylistFile{
				IsInitialized: true,
				StreamClips:   []*StreamClip{{Length: 3600, AngleIndex: 0}}, // 1 hour
				Settings:      settings.Settings{FilterShortPlaylists: true, FilterShortPlaylistsVal: 20},
			},
			want: true,
		},
		{
			name: "too short",
			playlist: &PlaylistFile{
				IsInitialized: true,
				StreamClips:   []*StreamClip{{Length: 10, AngleIndex: 0}}, // 10 seconds
				Settings:      settings.Settings{FilterShortPlaylists: true, FilterShortPlaylistsVal: 20},
			},
			want: false,
		},
		{
			name: "short filter disabled",
			playlist: &PlaylistFile{
				IsInitialized: true,
				StreamClips:   []*StreamClip{{Length: 10, AngleIndex: 0}},
				Settings:      settings.Settings{FilterShortPlaylists: false, FilterShortPlaylistsVal: 20},
			},
			want: true,
		},
		{
			name: "has loops with filter enabled",
			playlist: &PlaylistFile{
				IsInitialized: true,
				HasLoops:      true,
				StreamClips:   []*StreamClip{{Length: 3600, AngleIndex: 0}},
				Settings:      settings.Settings{FilterLoopingPlaylists: true},
			},
			want: false,
		},
		{
			name: "has loops with filter disabled",
			playlist: &PlaylistFile{
				IsInitialized: true,
				HasLoops:      true,
				StreamClips:   []*StreamClip{{Length: 3600, AngleIndex: 0}},
				Settings:      settings.Settings{FilterLoopingPlaylists: false},
			},
			want: true,
		},
		{
			name: "not initialized",
			playlist: &PlaylistFile{
				IsInitialized: false,
				StreamClips:   []*StreamClip{{Length: 3600, AngleIndex: 0}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.playlist.IsValid()
			if got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPlaylistFile_TotalBitRate tests bitrate calculation
func TestPlaylistFile_TotalBitRate(t *testing.T) {
	tests := []struct {
		name     string
		playlist *PlaylistFile
		expected uint64
	}{
		{
			name: "zero length",
			playlist: &PlaylistFile{
				StreamClips: []*StreamClip{
					{Length: 0, AngleIndex: 0},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.playlist.TotalBitRate()
			if got != tt.expected {
				t.Errorf("TotalBitRate() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestPlaylistFile_Scan_Integration tests parsing with a real MPLS file
// This test requires a BDMV folder structure to be set via environment variable
//
// To run this test, set the BDINFO_TEST_PATH environment variable:
//
//	Windows PowerShell: $env:BDINFO_TEST_PATH="D:\Movies\SomeBluray"
//	Windows CMD:        set BDINFO_TEST_PATH=D:\Movies\SomeBluray
//	Linux/Mac:          export BDINFO_TEST_PATH="/path/to/bluray"
//
// Then run: go test -v ./internal/bdrom -run TestPlaylistFile_Scan_Integration
func TestPlaylistFile_Scan_Integration(t *testing.T) {
	bdPath := os.Getenv("BDINFO_TEST_PATH")
	if bdPath == "" {
		t.Skip("BDINFO_TEST_PATH not set, skipping integration test")
	}

	playlistPath := filepath.Join(bdPath, "BDMV", "PLAYLIST")
	if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
		t.Skipf("Playlist directory not found: %s", playlistPath)
	}

	t.Logf("Testing with Blu-ray at: %s", bdPath)

	rom, err := New(bdPath, settings.Default("."))
	if err != nil {
		t.Fatalf("Failed to open BD structure: %v", err)
	}
	defer rom.Close()

	if len(rom.PlaylistFiles) == 0 {
		t.Fatal("No playlist files found")
	}

	// Test scanning first playlist
	var firstPlaylist *PlaylistFile
	for _, pl := range rom.PlaylistFiles {
		firstPlaylist = pl
		break
	}

	err = firstPlaylist.Scan(rom.StreamFiles, rom.StreamClipFiles)
	if err != nil {
		t.Fatalf("Failed to scan playlist %s: %v", firstPlaylist.Name, err)
	}

	t.Logf("Playlist: %s", firstPlaylist.Name)
	t.Logf("  Type: %s", firstPlaylist.FileType)
	t.Logf("  Clips: %d", len(firstPlaylist.StreamClips))
	t.Logf("  Duration: %.1f seconds", firstPlaylist.TotalLength())
	t.Logf("  File Size: %d bytes", firstPlaylist.FileSize())

	if len(firstPlaylist.StreamClips) == 0 {
		t.Error("Expected at least one stream clip")
	}

	if firstPlaylist.FileType == "" {
		t.Error("FileType should not be empty")
	}
}
