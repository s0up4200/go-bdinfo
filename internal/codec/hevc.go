package codec

import (
	"fmt"
	"math"

	"github.com/autobrr/go-bdinfo/internal/buffer"
	"github.com/autobrr/go-bdinfo/internal/stream"
)

func ScanHEVC(v *stream.VideoStream, data []byte) {
	// scan for start codes
	for i := 0; i+4 < len(data); i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			nalType := (data[i+3] >> 1) & 0x3f
			if nalType != 33 { // SPS
				continue
			}
			payload := data[i+4:]
			rbsp := RemoveEmulationBytes(payload)
			br := buffer.NewBitReader(rbsp)
			// skip NAL header (already consumed)
			_, _ = br.ReadBits(4) // sps_video_parameter_set_id
			maxSubLayersMinus1, ok := br.ReadBits(3)
			if !ok {
				return
			}
			_, _ = br.ReadBits(1) // sps_temporal_id_nesting_flag

			profile, level := parseHEVCProfileTierLevel(br, int(maxSubLayersMinus1))
			if profile == "" || level == "" {
				return
			}
			v.EncodingProfile = fmt.Sprintf("%s Profile %s", profile, level)
			v.IsInitialized = true
			return
		}
	}
}

func parseHEVCProfileTierLevel(br *buffer.BitReader, maxSubLayersMinus1 int) (string, string) {
	_, ok := br.ReadBits(2) // general_profile_space
	if !ok {
		return "", ""
	}
	_, _ = br.ReadBits(1) // general_tier_flag
	profileIDC, ok := br.ReadBits(5)
	if !ok {
		return "", ""
	}
	// compatibility flags 32 bits
	_, _ = br.ReadBits(32)
	// constraint flags 48 bits
	_, _ = br.ReadBits(48)
	levelIDC, ok := br.ReadBits(8)
	if !ok {
		return "", ""
	}

	profile := "Main"
	switch profileIDC {
	case 1:
		profile = "Main"
	case 2:
		profile = "Main 10"
	case 3:
		profile = "Main Still Picture"
	default:
		profile = "HEVC"
	}

	level := ""
	lvl := float64(levelIDC) / 30.0
	if math.Mod(lvl, 1.0) == 0 {
		level = fmt.Sprintf("%.0f", lvl)
	} else {
		level = fmt.Sprintf("%.1f", lvl)
	}

	// skip sub-layer profile/level bits
	if maxSubLayersMinus1 > 0 {
		for i := 0; i < maxSubLayersMinus1; i++ {
			_, _ = br.ReadBits(1) // sub_layer_profile_present_flag
			_, _ = br.ReadBits(1) // sub_layer_level_present_flag
		}
		if maxSubLayersMinus1 < 8 {
			_, _ = br.ReadBits(2 * (8 - maxSubLayersMinus1))
		}
		for i := 0; i < maxSubLayersMinus1; i++ {
			// ignore sub-layer profile/level
			_, _ = br.ReadBits(88)
			_, _ = br.ReadBits(8)
		}
	}
	return profile, level
}
