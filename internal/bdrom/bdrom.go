package bdrom

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/autobrr/go-bdinfo/internal/settings"
	"github.com/autobrr/go-bdinfo/internal/stream"
	"github.com/autobrr/go-bdinfo/internal/util"
)

type BDROM struct {
	Path string
	Settings settings.Settings

	DirectoryRoot string
	DirectoryBDMV string
	DirectoryBDJO string
	DirectoryCLIPINF string
	DirectoryPLAYLIST string
	DirectorySNP string
	DirectorySSIF string
	DirectorySTREAM string
	DirectoryMeta string

	VolumeLabel string
	DiscTitle string
	Size uint64
	IsBDPlus bool
	IsBDJava bool
	IsDBOX bool
	IsPSP bool
	Is3D bool
	Is50Hz bool
	IsUHD bool

	PlaylistFiles map[string]*PlaylistFile
	StreamClipFiles map[string]*StreamClipFile
	StreamFiles map[string]*StreamFile
	InterleavedFiles map[string]*InterleavedFile

	cleanup func()
}

type ScanResult struct {
	ScanError error
	FileErrors map[string]error
}

func New(path string, settings settings.Settings) (*BDROM, error) {
	root := path
	cleanup := func() {}
	if strings.HasSuffix(strings.ToLower(path), ".iso") {
		mount, unmount, err := mountISO(path)
		if err != nil {
			return nil, err
		}
		root = mount
		cleanup = unmount
	}

	bdmv, err := findBDMV(root)
	if err != nil {
		cleanup()
		return nil, err
	}

	rom := &BDROM{
		Path: path,
		Settings: settings,
		DirectoryBDMV: bdmv,
		DirectoryRoot: filepath.Dir(bdmv),
		PlaylistFiles: make(map[string]*PlaylistFile),
		StreamClipFiles: make(map[string]*StreamClipFile),
		StreamFiles: make(map[string]*StreamFile),
		InterleavedFiles: make(map[string]*InterleavedFile),
		cleanup: cleanup,
	}

	rom.DirectoryBDJO = findDirectory(bdmv, "BDJO")
	rom.DirectoryCLIPINF = findDirectory(bdmv, "CLIPINF")
	rom.DirectoryPLAYLIST = findDirectory(bdmv, "PLAYLIST")
	rom.DirectorySTREAM = findDirectory(bdmv, "STREAM")
	rom.DirectorySSIF = findDirectory(rom.DirectorySTREAM, "SSIF")
	rom.DirectoryMeta = findDirectory(bdmv, "META")
	rom.DirectorySNP = findDirectory(rom.DirectoryRoot, "SNP")

	if rom.DirectoryCLIPINF == "" || rom.DirectoryPLAYLIST == "" {
		rom.cleanup()
		return nil, fmt.Errorf("unable to locate BD structure")
	}

	rom.VolumeLabel = filepath.Base(rom.DirectoryRoot)
	rom.Size = uint64(getDirectorySize(rom.DirectoryRoot))

	indexPath := filepath.Join(rom.DirectoryBDMV, "index.bdmv")
	if data, err := os.ReadFile(indexPath); err == nil && len(data) >= 8 {
		if string(data[:8]) == "INDX0300" {
			rom.IsUHD = true
		}
	}

	rom.IsBDPlus = directoryExists(filepath.Join(rom.DirectoryRoot, "BDSVM")) ||
		directoryExists(filepath.Join(rom.DirectoryRoot, "SLYVM")) ||
		directoryExists(filepath.Join(rom.DirectoryRoot, "ANYVM"))

	if rom.DirectoryBDJO != "" {
		entries, _ := os.ReadDir(rom.DirectoryBDJO)
		if len(entries) > 0 {
			rom.IsBDJava = true
		}
	}

	if rom.DirectorySNP != "" {
		entries, _ := os.ReadDir(rom.DirectorySNP)
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			if strings.HasSuffix(name, ".mnv") {
				rom.IsPSP = true
				break
			}
		}
	}

	if rom.DirectorySSIF != "" {
		entries, _ := os.ReadDir(rom.DirectorySSIF)
		if len(entries) > 0 {
			rom.Is3D = true
		}
	}

	if fileExists(filepath.Join(rom.DirectoryRoot, "FilmIndex.xml")) {
		rom.IsDBOX = true
	}

	rom.DiscTitle = readDiscTitle(rom.DirectoryMeta)

	// load files
	if rom.DirectoryPLAYLIST != "" {
		files, _ := filepath.Glob(filepath.Join(rom.DirectoryPLAYLIST, "*.mpls"))
		if len(files) == 0 {
			files, _ = filepath.Glob(filepath.Join(rom.DirectoryPLAYLIST, "*.MPLS"))
		}
		for _, file := range files {
			pl := NewPlaylistFile(file, settings)
			rom.PlaylistFiles[pl.Name] = pl
		}
	}

	if rom.DirectorySTREAM != "" {
		files, _ := filepath.Glob(filepath.Join(rom.DirectorySTREAM, "*.m2ts"))
		if len(files) == 0 {
			files, _ = filepath.Glob(filepath.Join(rom.DirectorySTREAM, "*.M2TS"))
		}
		for _, file := range files {
			sf := NewStreamFile(file)
			rom.StreamFiles[sf.Name] = sf
		}
	}

	if rom.DirectoryCLIPINF != "" {
		files, _ := filepath.Glob(filepath.Join(rom.DirectoryCLIPINF, "*.clpi"))
		if len(files) == 0 {
			files, _ = filepath.Glob(filepath.Join(rom.DirectoryCLIPINF, "*.CLPI"))
		}
		for _, file := range files {
			cf := NewStreamClipFile(file)
			rom.StreamClipFiles[cf.Name] = cf
		}
	}

	if rom.DirectorySSIF != "" {
		files, _ := filepath.Glob(filepath.Join(rom.DirectorySSIF, "*.ssif"))
		if len(files) == 0 {
			files, _ = filepath.Glob(filepath.Join(rom.DirectorySSIF, "*.SSIF"))
		}
		for _, file := range files {
			info, _ := os.Stat(file)
			rom.InterleavedFiles[strings.ToUpper(filepathBase(file))] = &InterleavedFile{Path: file, Name: strings.ToUpper(filepathBase(file)), Size: info.Size()}
		}
	}

	return rom, nil
}

func (b *BDROM) Close() {
	if b.cleanup != nil {
		b.cleanup()
	}
}

func (b *BDROM) Scan() ScanResult {
	result := ScanResult{FileErrors: make(map[string]error)}

	for _, clip := range b.StreamClipFiles {
		if err := clip.Scan(); err != nil {
			result.FileErrors[clip.Name] = err
		}
	}

	for _, streamFile := range b.StreamFiles {
		ssifName := strings.ToUpper(strings.TrimSuffix(streamFile.Name, ".M2TS") + ".SSIF")
		if ssif, ok := b.InterleavedFiles[ssifName]; ok {
			streamFile.InterleavedFile = ssif
		}
	}

	for _, playlist := range b.PlaylistFiles {
		if err := playlist.Scan(b.StreamFiles, b.StreamClipFiles); err != nil {
			result.FileErrors[playlist.Name] = err
		}
	}

	// scan stream files
	for _, streamFile := range b.StreamFiles {
		var playlists []*PlaylistFile
		for _, playlist := range b.PlaylistFiles {
			for _, clip := range playlist.StreamClips {
				if clip.StreamFile == streamFile {
					playlists = append(playlists, playlist)
					break
				}
			}
		}
		if err := streamFile.Scan(playlists); err != nil {
			result.FileErrors[streamFile.Name] = err
		}
	}

	for _, playlist := range b.PlaylistFiles {
		playlist.Initialize()
		if b.Is50Hz {
			continue
		}
		vidCount := len(playlist.VideoStreams)
		for _, vs := range playlist.VideoStreams {
			if vs.FrameRate() == stream.FrameRate25 || vs.FrameRate() == stream.FrameRate50 {
				b.Is50Hz = true
			}
			if vidCount > 1 && b.Is3D {
				if vs.StreamType == stream.StreamTypeAVCVideo && playlist.MVCBaseViewR {
					base := true
					vs.BaseView = &base
				} else if vs.StreamType == stream.StreamTypeMVCVideo && !playlist.MVCBaseViewR {
					base := true
					vs.BaseView = &base
				} else if vs.StreamType == stream.StreamTypeAVCVideo || vs.StreamType == stream.StreamTypeMVCVideo {
					base := false
					vs.BaseView = &base
				}
			}
		}
	}

	return result
}

func findBDMV(root string) (string, error) {
	if strings.EqualFold(filepath.Base(root), "BDMV") {
		return root, nil
	}
	found := ""
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if d.IsDir() && strings.EqualFold(d.Name(), "BDMV") {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("unable to locate BD structure")
	}
	return found, nil
}

func findDirectory(parent, name string) string {
	if parent == "" {
		return ""
	}
	candidate := filepath.Join(parent, name)
	if directoryExists(candidate) {
		return candidate
	}
	entries, _ := os.ReadDir(parent)
	for _, entry := range entries {
		if entry.IsDir() && strings.EqualFold(entry.Name(), name) {
			return filepath.Join(parent, entry.Name())
		}
	}
	return ""
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func getDirectorySize(path string) int64 {
	var size int64
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.EqualFold(filepath.Ext(d.Name()), ".SSIF") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".SSIF") {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			size += info.Size()
		}
		return nil
	})
	return size
}

type discInfo struct {
	XMLName xml.Name `xml:"discinfo"`
	Title struct {
		Name string `xml:"name"`
	} `xml:"title"`
}

func readDiscTitle(metaDir string) string {
	if metaDir == "" {
		return ""
	}
	var target string
	_ = filepath.WalkDir(metaDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "bdmt_eng.xml") {
			target = path
			return filepath.SkipDir
		}
		return nil
	})
	if target == "" {
		return ""
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return ""
	}
	var doc struct {
		XMLName xml.Name `xml:"discinfo"`
		Title struct {
			Name string `xml:"name"`
		} `xml:"title"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return ""
	}
	name := strings.TrimSpace(doc.Title.Name)
	if strings.EqualFold(name, "blu-ray") {
		return ""
	}
	return name
}

func mountISO(path string) (string, func(), error) {
	if runtime.GOOS != "darwin" {
		return "", func() {}, fmt.Errorf("iso mounting not supported on %s", runtime.GOOS)
	}
	cmd := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", path)
	out, err := cmd.Output()
	if err != nil {
		return "", func() {}, fmt.Errorf("hdiutil attach failed: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	mount := ""
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.HasPrefix(fields[len(fields)-1], "/Volumes/") {
			mount = fields[len(fields)-1]
			break
		}
	}
	if mount == "" {
		return "", func() {}, fmt.Errorf("unable to find mount point")
	}
	cleanup := func() {
		_ = exec.Command("hdiutil", "detach", mount).Run()
	}
	return mount, cleanup, nil
}

func (b *BDROM) FormatSize() string {
	return util.FormatFileSize(float64(b.Size), true)
}
