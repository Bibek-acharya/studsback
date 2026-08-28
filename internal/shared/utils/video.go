package utils

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Supported video MIME types for iOS/Chrome/Firefox compatibility
var supportedVideoMIMETypes = map[string]bool{
	"video/mp4":     true, // H.264/H.265 MP4
	"video/webm":    true, // VP8/VP9 WebM
	"video/quicktime": true, // MOV (H.264)
}

// isH264MP4 checks if video data is an H.264 MP4 (ftyp box with compatible brand)
func isH264MP4(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	// Check for ftyp box at offset 4
	if string(data[4:8]) != "ftyp" {
		return false
	}
	brand := string(data[8:12])
	compatible := []string{"isom", "iso2", "mp41", "mp42", "avc1", "dash", "msdh", "msix", "f4v "}
	for _, b := range compatible {
		if brand == b {
			return true
		}
	}
	return false
}

// isWebM checks if video data is a WebM file (EBML header)
func isWebM(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// WebM starts with EBML header: 0x1A 0x45 0xDF 0xA3
	return data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3
}

// isMOV checks if video data is a QuickTime MOV file
func isMOV(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	if string(data[4:8]) != "ftyp" {
		return false
	}
	brand := string(data[8:12])
	compatible := []string{"qt  ", "mqt ", "isom", "iso2"}
	for _, b := range compatible {
		if brand == b {
			return true
		}
	}
	return false
}

// detectVideoFormat returns the detected format and whether it's already H.264 MP4
func detectVideoFormat(data []byte) (format string, isH264 bool) {
	if isH264MP4(data) {
		return "mp4", true
	}
	if isMOV(data) {
		return "mov", false // MOV needs transcoding
	}
	if isWebM(data) {
		return "webm", false
	}
	return "unknown", false
}

// ValidateVideo checks if the uploaded video bytes are a supported format
func ValidateVideo(data []byte) error {
	format, _ := detectVideoFormat(data)
	if format == "unknown" {
		return fmt.Errorf("unsupported video format. Please upload MP4, WebM, or MOV files")
	}
	return nil
}

// TranscodeToH264MP4 converts video data to H.264 MP4 using FFmpeg.
// Returns the transcoded data. If already H.264 MP4, returns original data unchanged.
func TranscodeToH264MP4(data []byte) ([]byte, error) {
	_, isH264 := detectVideoFormat(data)
	if isH264 {
		return data, nil
	}

	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-f", "mp4",
		"pipe:1",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("video transcoding failed: %s", strings.TrimSpace(stderr.String()))
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("transcoding produced empty output")
	}

	return stdout.Bytes(), nil
}
