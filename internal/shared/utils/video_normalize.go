package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"studsphere/backend/internal/shared/config"
)

// Cross-platform video normalization.
//
// Admin uploads arrive as whatever the phone/camera produced (HEVC in .mp4,
// ProRes/MOV, VP9 in .mkv, MJPEG in .avi, ...), none of which every desktop and
// mobile browser can play. This helper re-encodes the upload into the single
// combination that is broadly playable: H.264 (libx264) video in yuv420p with
// AAC audio, moov atom up front (faststart) and a 2 second GOP.
//
// Operational rules enforced here:
//   - ffmpeg is invoked with a bounded context (configurable timeout) and is
//     killed when the deadline passes;
//   - the upload is streamed to a temp file and ffmpeg writes to a temp file;
//     the upload is never held in memory and ffmpeg's stdout/stderr is never
//     buffered in memory either (stderr goes to a temp file, of which only a
//     bounded tail is read back for diagnostics);
//   - every temp file is removed on every code path, including failures;
//   - a missing ffmpeg binary is reported as ErrFFmpegUnavailable so callers can
//     answer 503 instead of storing bytes they cannot guarantee are playable.

var (
	// ErrFFmpegUnavailable means the ffmpeg binary could not be found.
	ErrFFmpegUnavailable = errors.New("video normalization is unavailable: ffmpeg is not installed on this server")
	// ErrVideoNormalizeFailed means ffmpeg ran but could not produce an MP4.
	ErrVideoNormalizeFailed = errors.New("video could not be converted to a playable MP4")
	// ErrVideoNormalizeTimeout means the bounded context expired first.
	ErrVideoNormalizeTimeout = errors.New("video conversion timed out")
)

// FFmpegBinary is the executable this package shells out to.
const FFmpegBinary = "ffmpeg"

// NormalizedVideoContentType is the content type of the normalized artifact.
const NormalizedVideoContentType = "video/mp4"

// NormalizedVideoExtension is the extension of the normalized artifact.
const NormalizedVideoExtension = ".mp4"

// DefaultVideoTranscodeTimeout is used when the config helper is unavailable
// (for example in unit tests).
const DefaultVideoTranscodeTimeout = 10 * time.Minute

// maxFfmpegLogBytes bounds how much of ffmpeg's stderr tail is read back into an
// error message. The log itself always lives in a temp file.
const maxFfmpegLogBytes = 8 * 1024

// NormalizedVideo describes the temp artifact produced by NormalizeVideoToMP4.
type NormalizedVideo struct {
	// Path is the temp file holding the normalized MP4. It is owned by the
	// caller and MUST be released with Cleanup (defer immediately).
	Path string
	// Size is the size in bytes of the normalized MP4.
	Size int64
	// ContentType is always video/mp4.
	ContentType string
	// SourceSize is how many bytes were streamed from the upload.
	SourceSize int64
	// Duration is the wall-clock time ffmpeg took.
	Duration time.Duration
}

// Cleanup removes the normalized artifact and every temp file it was built in.
// It is safe to call more than once.
func (v *NormalizedVideo) Cleanup() {
	if v == nil || v.Path == "" {
		return
	}
	// The artifact lives in a dedicated temp directory, so removing the
	// directory clears the artifact and any sibling leftovers too.
	_ = os.RemoveAll(filepath.Dir(v.Path))
	v.Path = ""
}

// VideoTranscodeTimeout returns the configured bound for one ffmpeg run.
func VideoTranscodeTimeout() time.Duration {
	if config.AppConfig != nil && config.AppConfig.StudyResourceVideoTranscodeTimeout > 0 {
		return config.AppConfig.StudyResourceVideoTranscodeTimeout
	}
	return DefaultVideoTranscodeTimeout
}

// FFMPEGAvailable reports whether the ffmpeg binary can be found on PATH.
func FFMPEGAvailable() bool {
	_, err := exec.LookPath(FFmpegBinary)
	return err == nil
}

// NormalizeVideoToMP4 converts a supported source container into a broadly
// playable H.264/AAC MP4 on a temp file.
//
// src is streamed to a temp file rather than piped into ffmpeg's stdin on
// purpose: several source containers (MP4/MOV with a trailing moov atom, MKV
// with cues at the end) need seeking, which a pipe cannot provide. The upload
// still never lands in memory.
func NormalizeVideoToMP4(ctx context.Context, src io.Reader) (*NormalizedVideo, error) {
	ffmpegPath, err := exec.LookPath(FFmpegBinary)
	if err != nil {
		return nil, ErrFFmpegUnavailable
	}
	if src == nil {
		return nil, errors.New("no video data provided")
	}

	workDir, err := os.MkdirTemp("", "studsphere-video-")
	if err != nil {
		return nil, fmt.Errorf("failed to create video work directory: %w", err)
	}
	// On every failure path the whole work directory goes away; on success the
	// caller releases it through NormalizedVideo.Cleanup.
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(workDir)
		}
	}()

	// 1. Stream the upload to disk.
	inputPath := filepath.Join(workDir, "source")
	input, err := os.Create(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to buffer video upload: %w", err)
	}
	sourceSize, copyErr := io.Copy(input, src)
	closeErr := input.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("failed to buffer video upload: %w", copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("failed to buffer video upload: %w", closeErr)
	}
	if sourceSize == 0 {
		return nil, errors.New("uploaded video is empty")
	}

	// 2. ffmpeg writes the normalized artifact to a temp file; its stderr goes
	//    to another temp file instead of an in-memory buffer.
	outputPath := filepath.Join(workDir, "normalized"+NormalizedVideoExtension)
	logPath := filepath.Join(workDir, "ffmpeg.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create ffmpeg log file: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, VideoTranscodeTimeout())
	defer cancel()

	started := time.Now()
	cmd := exec.CommandContext(runCtx, ffmpegPath, videoNormalizeArgs(inputPath, outputPath)...)
	cmd.Stderr = logFile

	runErr := cmd.Run()
	elapsed := time.Since(started)
	_ = logFile.Close()

	if runErr != nil {
		tail := readFileTail(logPath, maxFfmpegLogBytes)
		switch {
		case errors.Is(runCtx.Err(), context.DeadlineExceeded):
			return nil, fmt.Errorf("%w after %s", ErrVideoNormalizeTimeout, VideoTranscodeTimeout())
		case errors.Is(runCtx.Err(), context.Canceled):
			return nil, fmt.Errorf("%w: conversion canceled", ErrVideoNormalizeFailed)
		default:
			return nil, fmt.Errorf("%w: ffmpeg failed after %s: %s", ErrVideoNormalizeFailed, elapsed.Round(time.Second), tail)
		}
	}

	stat, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("%w: normalized file is missing", ErrVideoNormalizeFailed)
	}
	if stat.Size() == 0 {
		return nil, fmt.Errorf("%w: normalized file is empty", ErrVideoNormalizeFailed)
	}

	success = true
	return &NormalizedVideo{
		Path:        outputPath,
		Size:        stat.Size(),
		ContentType: NormalizedVideoContentType,
		SourceSize:  sourceSize,
		Duration:    elapsed,
	}, nil
}

// videoNormalizeArgs builds the ffmpeg argument list for a broadly playable
// MP4. Autorotation is intentionally left at ffmpeg's default (enabled), so a
// phone video stored with a rotation matrix plays upright.
func videoNormalizeArgs(inputPath, outputPath string) []string {
	return []string{
		"-hide_banner",
		"-nostdin",
		"-loglevel", "error",
		"-y",
		"-i", inputPath,
		// First video and (optional) first audio stream only.
		"-map", "0:v:0",
		"-map", "0:a:0?",
		// H.264 High@4.0 in the universally supported 8-bit 4:2:0 pixel format.
		"-c:v", "libx264",
		"-profile:v", "high",
		"-level", "4.0",
		"-pix_fmt", "yuv420p",
		"-preset", "veryfast",
		"-crf", "23",
		// Even dimensions: yuv420p cannot represent odd width/height, and phone
		// captures are frequently odd on one axis. setrange=tv pins limited
		// range so a full-range source (JPEG/MJPEG) still comes out as plain
		// yuv420p rather than the less portable yuvj420p.
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2,setrange=tv",
		// 2 second GOP keeps seeking responsive on the web player.
		"-g", "50",
		"-keyint_min", "50",
		"-sc_threshold", "0",
		// AAC-LC stereo when the source carries audio.
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		// moov first: playback can start before the whole file is read.
		"-movflags", "+faststart",
		// Drop container/stream metadata and encoder version strings.
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-fflags", "+bitexact",
		"-flags:v", "+bitexact",
		"-flags:a", "+bitexact",
		outputPath,
	}
}

// readFileTail returns at most limit trailing bytes of a file, for diagnostics.
func readFileTail(path string, limit int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if limit > 0 && len(text) > limit {
		text = "..." + text[len(text)-limit:]
	}
	return text
}
