package bdrom

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/autobrr/go-bdinfo/internal/codec"
	"github.com/autobrr/go-bdinfo/internal/settings"
	"github.com/autobrr/go-bdinfo/internal/stream"
)

const (
	maxStreamData = 1024 * 1024
	maxScanBytes  = 100 * 1024 * 1024
)

type InterleavedFile struct {
	Path string
	Name string
	Size int64
}

type StreamFile struct {
	Path string
	Name string
	Size int64
	Length float64
	InterleavedFile *InterleavedFile
	Streams map[uint16]stream.Info
}

func NewStreamFile(path string) *StreamFile {
	return &StreamFile{
		Path: path,
		Name: strings.ToUpper(filepathBase(path)),
		Streams: make(map[uint16]stream.Info),
	}
}

func (s *StreamFile) DisplayName(settings settings.Settings) string {
	if settings.EnableSSIF && s.InterleavedFile != nil {
		return s.InterleavedFile.Name
	}
	return s.Name
}

func (s *StreamFile) Scan(playlists []*PlaylistFile) error {
	if s.Path == "" {
		return nil
	}
	// ensure streams map populated from clip info
	if len(s.Streams) == 0 {
		for _, pl := range playlists {
			for _, clip := range pl.StreamClips {
				if clip.StreamFile == s && clip.StreamClipFile != nil {
					for pid, st := range clip.StreamClipFile.Streams {
						if _, ok := s.Streams[pid]; !ok {
							s.Streams[pid] = st.Clone()
						}
					}
				}
			}
		}
	}

	// compute length from playlists
	length := 0.0
	for _, pl := range playlists {
		for _, clip := range pl.StreamClips {
			if clip.StreamFile == s && clip.AngleIndex == 0 {
				if clip.Length > length {
					length = clip.Length
				}
			}
		}
	}
	s.Length = length

	f, err := os.Open(s.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err == nil {
		s.Size = info.Size()
	}

	reader := bufio.NewReaderSize(f, 1<<20)
	first := make([]byte, 192)
	if _, err := io.ReadFull(reader, first); err != nil {
		return err
	}

	packetSize := 192
	syncOffset := 4
	if first[0] == 0x47 {
		packetSize = 188
		syncOffset = 0
	} else if first[4] == 0x47 {
		packetSize = 192
		syncOffset = 4
	} else {
		return fmt.Errorf("invalid TS sync for %s", s.Name)
	}

	buffers := map[uint16][]byte{}
	packetCounts := map[uint16]uint64{}
	payloadBytes := map[uint16]uint64{}

	processed := int64(0)
	processPacket := func(pkt []byte) {
		if len(pkt) <= syncOffset || pkt[syncOffset] != 0x47 {
			return
		}
		pid := (uint16(pkt[syncOffset+1]&0x1f) << 8) | uint16(pkt[syncOffset+2])
		if _, ok := s.Streams[pid]; !ok {
			return
		}
		payloadStart := (pkt[syncOffset+1] & 0x40) != 0
		adaptation := (pkt[syncOffset+3] >> 4) & 0x3
		idx := syncOffset + 4
		if adaptation == 0 || adaptation == 2 {
			return
		}
		if adaptation == 3 {
			if idx >= len(pkt) {
				return
			}
			adapLen := int(pkt[idx])
			idx += 1 + adapLen
		}
		if idx >= len(pkt) {
			return
		}
		payload := pkt[idx:]
		if payloadStart && len(payload) > 9 {
			if payload[0] == 0x00 && payload[1] == 0x00 && payload[2] == 0x01 {
				hdrLen := int(payload[8])
				if 9+hdrLen < len(payload) {
					payload = payload[9+hdrLen:]
				} else {
					payload = nil
				}
			}
		}
		if payload == nil {
			return
		}
		packetCounts[pid]++
		payloadBytes[pid] += uint64(len(payload))
		if len(buffers[pid]) < maxStreamData {
			need := maxStreamData - len(buffers[pid])
			if len(payload) > need {
				payload = payload[:need]
			}
			buffers[pid] = append(buffers[pid], payload...)
		}
	}

	processPacket(first[:packetSize])
	processed += int64(packetSize)

	buf := make([]byte, packetSize)
	for {
		if processed >= maxScanBytes {
			break
		}
		n, err := io.ReadFull(reader, buf)
		if err != nil {
			break
		}
		processed += int64(n)
		processPacket(buf[:packetSize])
		complete := true
		for pid := range s.Streams {
			if len(buffers[pid]) < maxStreamData {
				complete = false
				break
			}
		}
		if complete {
			break
		}
	}

	for pid, st := range s.Streams {
		st.Base().PacketCount = packetCounts[pid]
		st.Base().PayloadBytes = payloadBytes[pid]
		if s.Length > 0 {
			st.Base().PacketSeconds = s.Length
			st.Base().BitRate = int64(float64(payloadBytes[pid]) * 8.0 / s.Length)
		}
		data := buffers[pid]
		switch concrete := st.(type) {
		case *stream.VideoStream:
			switch concrete.StreamType {
			case stream.StreamTypeAVCVideo:
				codec.ScanAVC(concrete, data, nil)
			case stream.StreamTypeHEVCVideo:
				codec.ScanHEVC(concrete, data)
			case stream.StreamTypeMPEG2Video:
				codec.ScanMPEG2(concrete, data)
			case stream.StreamTypeVC1Video:
				codec.ScanVC1(concrete, data)
			}
		case *stream.AudioStream:
			switch concrete.StreamType {
			case stream.StreamTypeAC3Audio, stream.StreamTypeAC3PlusAudio, stream.StreamTypeAC3PlusSecondaryAudio:
				codec.ScanAC3(concrete, data)
			case stream.StreamTypeAC3TrueHDAudio:
				codec.ScanTrueHD(concrete, data)
			case stream.StreamTypeDTSAudio:
				codec.ScanDTS(concrete, data, int64(concrete.BitRate))
			case stream.StreamTypeDTSHDAudio, stream.StreamTypeDTSHDMasterAudio, stream.StreamTypeDTSHDSecondaryAudio:
				codec.ScanDTSHD(concrete, data, int64(concrete.BitRate))
			case stream.StreamTypeLPCMAudio:
				codec.ScanLPCM(concrete, data)
			case stream.StreamTypeMPEG2AACAudio, stream.StreamTypeMPEG4AACAudio:
				codec.ScanAAC(concrete, data)
			}
		case *stream.GraphicsStream:
			codec.ScanPGS(concrete, data)
		}
	}

	// update clips with packet counts based on length
	if s.Length > 0 && s.Size > 0 {
		totalPackets := uint64(s.Size / int64(packetSize))
		for _, pl := range playlists {
			for _, clip := range pl.StreamClips {
				if clip.StreamFile == s {
					ratio := clip.Length / s.Length
					if ratio > 1 {
						ratio = 1
					}
					clip.PacketCount = uint64(float64(totalPackets) * ratio)
					clip.PacketSeconds = clip.Length
				}
			}
		}
	}

	return nil
}
