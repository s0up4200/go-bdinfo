package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/s0up4200/go-bdinfo/internal/bdrom"
	"github.com/s0up4200/go-bdinfo/internal/report"
	"github.com/s0up4200/go-bdinfo/internal/settings"
)

var version = "dev"

type optBool struct {
	set   bool
	value bool
}

func (o *optBool) Set(s string) error {
	if s == "" {
		o.value = true
		o.set = true
		return nil
	}
	if s == "true" || s == "1" {
		o.value = true
		o.set = true
		return nil
	}
	if s == "false" || s == "0" {
		o.value = false
		o.set = true
		return nil
	}
	return fmt.Errorf("invalid boolean %q", s)
}

func (o *optBool) String() string {
	if !o.set {
		return ""
	}
	if o.value {
		return "true"
	}
	return "false"
}

func (o *optBool) IsBoolFlag() bool { return true }

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("bdinfo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var pathLong string
	var pathShort string
	var reportFile string
	var filterShortValue int

	var genDiag optBool
	var extDiag optBool
	var enableSSIF optBool
	var filterLooping optBool
	var filterShort optBool
	var keepOrder optBool
	var genSummary optBool
	var includeNotes optBool
	var groupByTime optBool
	var forumsOnly optBool
	var mainOnly optBool
	var summaryOnly optBool
	var selfUpdate bool

	fs.StringVar(&pathLong, "path", "", "The path to iso or bluray folder")
	fs.StringVar(&pathShort, "p", "", "The path to iso or bluray folder")
	fs.Var(&genDiag, "generatestreamdiagnostics", "Generate the stream diagnostics")
	fs.Var(&genDiag, "g", "Generate the stream diagnostics")
	fs.Var(&extDiag, "extendedstreamdiagnostics", "Generate the extended stream diagnostics")
	fs.Var(&extDiag, "e", "Generate the extended stream diagnostics")
	fs.Var(&enableSSIF, "enablessif", "Enable SSIF support")
	fs.Var(&enableSSIF, "b", "Enable SSIF support")
	fs.Var(&filterLooping, "filterloopingplaylists", "Filter looping playlist")
	fs.Var(&filterLooping, "l", "Filter looping playlist")
	fs.Var(&filterShort, "filtershortplaylist", "Filter short playlist")
	fs.Var(&filterShort, "y", "Filter short playlist")
	fs.IntVar(&filterShortValue, "filtershortplaylistvalue", 20, "Filter number of short playlist")
	fs.IntVar(&filterShortValue, "v", 20, "Filter number of short playlist")
	fs.Var(&keepOrder, "keepstreamorder", "Keep stream order")
	fs.Var(&keepOrder, "k", "Keep stream order")
	fs.Var(&genSummary, "generatetextsummary", "Generate summary")
	fs.Var(&genSummary, "m", "Generate summary")
	fs.StringVar(&reportFile, "reportfilename", "", "The report filename with extension")
	fs.StringVar(&reportFile, "o", "", "The report filename with extension")
	fs.Var(&includeNotes, "includeversionandnotes", "Include version and notes inside report")
	fs.Var(&includeNotes, "q", "Include version and notes inside report")
	fs.Var(&groupByTime, "groupbytime", "Group by time")
	fs.Var(&groupByTime, "j", "Group by time")
	fs.Var(&forumsOnly, "forumsonly", "Output only the forums paste block")
	fs.Var(&forumsOnly, "f", "Output only the forums paste block")
	fs.Var(&mainOnly, "main", "Output only the main playlist (likely what you want)")
	fs.Var(&summaryOnly, "summaryonly", "Output only the quick summary block (likely what you want)")
	fs.Var(&summaryOnly, "s", "Output only the quick summary block (likely what you want)")
	fs.BoolVar(&selfUpdate, "self-update", false, "Update bdinfo to latest version")
	fs.BoolVar(&selfUpdate, "update", false, "Update bdinfo to latest version")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if selfUpdate {
		return runSelfUpdate(context.Background())
	}

	path := pathLong
	if path == "" {
		path = pathShort
	}
	if path == "" {
		return errors.New("path is required")
	}

	cwd, _ := os.Getwd()
	s := settings.Default(cwd)
	if genDiag.set {
		s.GenerateStreamDiagnostics = genDiag.value
	}
	if extDiag.set {
		s.ExtendedStreamDiagnostics = extDiag.value
	}
	if enableSSIF.set {
		s.EnableSSIF = enableSSIF.value
	}
	if filterLooping.set {
		s.FilterLoopingPlaylists = filterLooping.value
	}
	if filterShort.set {
		s.FilterShortPlaylists = filterShort.value
	}
	s.FilterShortPlaylistsVal = filterShortValue
	if keepOrder.set {
		s.KeepStreamOrder = keepOrder.value
	}
	if genSummary.set {
		s.GenerateTextSummary = genSummary.value
	}
	if reportFile != "" {
		s.ReportFileName = reportFile
	}
	if includeNotes.set {
		s.IncludeVersionAndNotes = includeNotes.value
	}
	if groupByTime.set {
		s.GroupByTime = groupByTime.value
	}
	if forumsOnly.set {
		s.ForumsOnly = forumsOnly.value
	}
	if mainOnly.set {
		s.MainPlaylistOnly = mainOnly.value
	}
	if summaryOnly.set {
		s.SummaryOnly = summaryOnly.value
	}

	return runForPath(path, s)
}

func runSelfUpdate(ctx context.Context) error {
	if version == "" || version == "dev" {
		return errors.New("self-update is only available in release builds")
	}

	if _, err := semver.ParseTolerant(version); err != nil {
		return fmt.Errorf("could not parse version: %w", err)
	}

	latest, found, err := selfupdate.DetectLatest(ctx, selfupdate.ParseSlug("s0up4200/go-bdinfo"))
	if err != nil {
		return fmt.Errorf("error occurred while detecting version: %w", err)
	}
	if !found {
		return fmt.Errorf("latest version for %s/%s could not be found from github repository", "s0up4200/go-bdinfo", version)
	}

	if latest.LessOrEqual(version) {
		fmt.Printf("Current binary is the latest version: %s\n", version)
		return nil
	}

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}

	if err := selfupdate.UpdateTo(ctx, latest.AssetURL, latest.AssetName, exe); err != nil {
		return fmt.Errorf("error occurred while updating binary: %w", err)
	}

	fmt.Printf("Successfully updated to version: %s\n", latest.Version())
	return nil
}

func runForPath(path string, settings settings.Settings) error {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".iso") {
		return scanAndReport(path, settings)
	}

	bdmvDirs := []string{}
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.EqualFold(d.Name(), "BDMV") {
			bdmvDirs = append(bdmvDirs, p)
			return filepath.SkipDir
		}
		return nil
	})
	isIsoLevel := false
	isoFiles := []string{}
	if len(bdmvDirs) == 0 {
		_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() && strings.HasSuffix(strings.ToLower(p), ".iso") {
				isoFiles = append(isoFiles, p)
			}
			return nil
		})
		if len(isoFiles) > 0 {
			isIsoLevel = true
			bdmvDirs = isoFiles
		}
	}

	if len(bdmvDirs) > 1 || isIsoLevel {
		oldReport := settings.ReportFileName
		reports := []string{}
		for _, sub := range bdmvDirs {
			target := sub
			if !isIsoLevel {
				target = filepath.Dir(sub)
			}
			if oldReport != "" {
				parent := filepath.Dir(target)
				if isIsoLevel {
					settings.ReportFileName = filepath.Join(parent, strings.TrimSuffix(filepath.Base(target), filepath.Ext(target))) + "." + strings.TrimPrefix(filepath.Ext(oldReport), ".")
				} else {
					settings.ReportFileName = filepath.Join(parent, filepath.Base(target)) + "." + strings.TrimPrefix(filepath.Ext(oldReport), ".")
				}
				reports = append(reports, settings.ReportFileName)
			}
			if err := scanAndReport(target, settings); err != nil {
				return err
			}
		}
		if oldReport != "" && len(reports) > 0 {
			if len(reports) == 1 {
				_ = os.Rename(reports[0], oldReport)
				return nil
			}
			combined, err := os.Create(oldReport)
			if err != nil {
				return err
			}
			defer combined.Close()
			for _, reportFile := range reports {
				data, err := os.ReadFile(reportFile)
				if err != nil {
					continue
				}
				combined.Write(data)
				combined.WriteString("\n\n\n\n\n")
				_ = os.Remove(reportFile)
			}
			return nil
		}
		return nil
	}

	return scanAndReport(path, settings)
}

func scanAndReport(path string, settings settings.Settings) error {
	rom, err := bdrom.New(path, settings)
	if err != nil {
		return err
	}
	defer rom.Close()

	result := rom.Scan()
	fullResult := rom.ScanFull()
	if fullResult.ScanError != nil {
		result.ScanError = fullResult.ScanError
	}
	for name, err := range fullResult.FileErrors {
		result.FileErrors[name] = err
	}

	playlists := make([]*bdrom.PlaylistFile, 0, len(rom.PlaylistFiles))
	if len(rom.PlaylistOrder) > 0 {
		for _, name := range rom.PlaylistOrder {
			if pl, ok := rom.PlaylistFiles[name]; ok {
				playlists = append(playlists, pl)
			}
		}
	} else {
		for _, pl := range rom.PlaylistFiles {
			playlists = append(playlists, pl)
		}
	}
	_, err = report.WriteReport("", rom, playlists, result, settings)
	return err
}
