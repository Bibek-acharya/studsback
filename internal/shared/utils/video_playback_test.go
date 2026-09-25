package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studsphere/backend/internal/shared/config"

	"github.com/golang-jwt/jwt/v5"
)

func withTestSecret(t *testing.T, secret string) {
	t.Helper()
	previous := config.AppConfig
	t.Cleanup(func() { config.AppConfig = previous })
	config.AppConfig = &config.Config{JWTSecret: secret}
}

// requireFFmpeg skips a test when the binary is missing, so the suite never
// depends on a locally installed ffmpeg.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if !FFMPEGAvailable() {
		t.Skipf("ffmpeg is not installed on this machine; skipping the normalization probe")
	}
}

// ------------------------------------------------------ capability probing

func TestFFmpegAvailabilityProbe(t *testing.T) {
	// The probe must agree with exec.LookPath and must never panic.
	_, err := exec.LookPath(FFmpegBinary)
	if got := FFMPEGAvailable(); got != (err == nil) {
		t.Errorf("FFMPEGAvailable() = %v, LookPath err = %v", got, err)
	}
}

func TestVideoTranscodeTimeoutDefaultsAndConfig(t *testing.T) {
	previous := config.AppConfig
	t.Cleanup(func() { config.AppConfig = previous })

	config.AppConfig = nil
	if got := VideoTranscodeTimeout(); got != DefaultVideoTranscodeTimeout {
		t.Errorf("timeout without config = %s, want %s", got, DefaultVideoTranscodeTimeout)
	}

	config.AppConfig = &config.Config{StudyResourceVideoTranscodeTimeout: 90 * time.Second}
	if got := VideoTranscodeTimeout(); got != 90*time.Second {
		t.Errorf("configured timeout = %s, want 90s", got)
	}

	// A non-positive configured value falls back to the default instead of
	// producing an instantly-expiring bound.
	config.AppConfig = &config.Config{StudyResourceVideoTranscodeTimeout: 0}
	if got := VideoTranscodeTimeout(); got != DefaultVideoTranscodeTimeout {
		t.Errorf("zero timeout = %s, want the default %s", got, DefaultVideoTranscodeTimeout)
	}
}

// Without ffmpeg the caller must be able to tell it apart from a bad upload.
func TestNormalizeReportsMissingFFmpeg(t *testing.T) {
	if FFMPEGAvailable() {
		t.Skip("ffmpeg is installed, so the unavailable branch cannot be exercised")
	}
	_, err := NormalizeVideoToMP4(context.Background(), strings.NewReader("not a video"))
	if !errors.Is(err, ErrFFmpegUnavailable) {
		t.Fatalf("error = %v, want ErrFFmpegUnavailable", err)
	}
}

func TestNormalizeRejectsNilAndEmptyInput(t *testing.T) {
	requireFFmpeg(t)

	if _, err := NormalizeVideoToMP4(context.Background(), nil); err == nil {
		t.Error("a nil source must be refused")
	}
	if _, err := NormalizeVideoToMP4(context.Background(), bytes.NewReader(nil)); err == nil {
		t.Error("an empty upload must be refused")
	}
}

func TestNormalizeRejectsGarbage(t *testing.T) {
	requireFFmpeg(t)

	_, err := NormalizeVideoToMP4(context.Background(), strings.NewReader("this is not a video file"))
	if err == nil {
		t.Fatal("garbage input must fail the conversion")
	}
	if !errors.Is(err, ErrVideoNormalizeFailed) && !errors.Is(err, ErrVideoNormalizeTimeout) {
		t.Errorf("error = %v, want a normalization failure", err)
	}
}

// A real conversion must produce a faststart H.264/yuv420p MP4, and every temp
// file must be gone afterwards.
func TestNormalizeProducesPlayableMP4(t *testing.T) {
	requireFFmpeg(t)

	source := makeTestVideo(t, "321x241", "mpeg4")

	normalized, err := NormalizeVideoToMP4(context.Background(), bytes.NewReader(source))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if normalized.ContentType != NormalizedVideoContentType {
		t.Errorf("content type = %q, want %q", normalized.ContentType, NormalizedVideoContentType)
	}
	if normalized.Size <= 0 {
		t.Errorf("size = %d, want > 0", normalized.Size)
	}
	if normalized.SourceSize != int64(len(source)) {
		t.Errorf("source size = %d, want %d", normalized.SourceSize, len(source))
	}
	if _, err := os.Stat(normalized.Path); err != nil {
		t.Fatalf("normalized artifact missing: %v", err)
	}
	if filepath.Ext(normalized.Path) != NormalizedVideoExtension {
		t.Errorf("artifact extension = %q, want %q", filepath.Ext(normalized.Path), NormalizedVideoExtension)
	}

	// yuv420p requires even dimensions; the source is deliberately odd.
	probe := probeVideo(t, normalized.Path)
	if probe.Video.Codec != "h264" {
		t.Errorf("video codec = %q, want h264", probe.Video.Codec)
	}
	if probe.Video.PixFmt != "yuv420p" {
		t.Errorf("pixel format = %q, want yuv420p", probe.Video.PixFmt)
	}
	if probe.Video.Width%2 != 0 || probe.Video.Height%2 != 0 {
		t.Errorf("dimensions = %dx%d, want even values for yuv420p", probe.Video.Width, probe.Video.Height)
	}
	if probe.Video.Codec != "h264" || probe.Audio.Codec != "aac" {
		t.Errorf("audio codec = %q, want aac", probe.Audio.Codec)
	}
	if !probe.Faststart {
		t.Error("moov atom is not at the front: playback would have to buffer the whole file")
	}

	// The whole work directory (artifact + buffered source + ffmpeg log) must
	// be gone after Cleanup, and Cleanup must be safe to repeat.
	workDir := filepath.Dir(normalized.Path)
	normalized.Cleanup()
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Errorf("work directory %s survived Cleanup", workDir)
	}
	normalized.Cleanup()
}

// Every source container an admin may upload must end up playable.
func TestNormalizeAcceptsCommonSourceContainers(t *testing.T) {
	requireFFmpeg(t)

	for _, codec := range []string{"mpeg4", "libx264", "mjpeg"} {
		t.Run(codec, func(t *testing.T) {
			source := makeTestVideo(t, "320x240", codec)
			normalized, err := NormalizeVideoToMP4(context.Background(), bytes.NewReader(source))
			if err != nil {
				t.Fatalf("normalize %s: %v", codec, err)
			}
			defer normalized.Cleanup()

			probe := probeVideo(t, normalized.Path)
			if probe.Video.Codec != "h264" || probe.Video.PixFmt != "yuv420p" {
				t.Errorf("%s normalized to %s/%s, want h264/yuv420p", codec, probe.Video.Codec, probe.Video.PixFmt)
			}
		})
	}
}

// A bounded context must stop the run and clean up.
func TestNormalizeHonorsContextCancellation(t *testing.T) {
	requireFFmpeg(t)

	source := makeTestVideo(t, "1920x1080", "libx265")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already expired

	_, err := NormalizeVideoToMP4(ctx, bytes.NewReader(source))
	if err == nil {
		t.Fatal("a cancelled context must abort the conversion")
	}
}

// ---------------------------------------------------------- playback tokens

func TestPlaybackTokenRoundTrip(t *testing.T) {
	withTestSecret(t, "test-secret")

	token, expiresAt, err := IssuePlaybackToken(11, 42, DefaultPlaybackTokenTTL)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if remaining := time.Until(expiresAt); remaining <= 0 || remaining > DefaultPlaybackTokenTTL+time.Minute {
		t.Errorf("expiry in %s, want ~%s", remaining, DefaultPlaybackTokenTTL)
	}

	claims, err := ParsePlaybackToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 11 || claims.ResourceID != 42 {
		t.Errorf("claims = %+v, want user 11 / resource 42", claims)
	}
	if claims.Purpose != PlaybackTokenPurpose {
		t.Errorf("purpose = %q, want %q", claims.Purpose, PlaybackTokenPurpose)
	}
	if claims.Scope() != PlaybackTokenAudience {
		t.Errorf("audience = %q, want %q", claims.Scope(), PlaybackTokenAudience)
	}
	if claims.Issuer != PlaybackTokenIssuer {
		t.Errorf("issuer = %q, want %q", claims.Issuer, PlaybackTokenIssuer)
	}
	if claims.ID == "" {
		t.Error("jti must be present so a grant can be traced")
	}

	// The jti is unique per grant.
	second, _, err := IssuePlaybackToken(11, 42, DefaultPlaybackTokenTTL)
	if err != nil {
		t.Fatalf("issue second: %v", err)
	}
	secondClaims, err := ParsePlaybackToken(second)
	if err != nil {
		t.Fatalf("parse second: %v", err)
	}
	if secondClaims.ID == claims.ID {
		t.Error("two grants must not share a jti")
	}
}

func TestPlaybackTokenIsNotASessionToken(t *testing.T) {
	withTestSecret(t, "test-secret")

	sessionToken, err := GenerateToken(11, "student@example.com", "student", 0)
	if err != nil {
		t.Fatalf("generate session token: %v", err)
	}
	if _, err := ParsePlaybackToken(sessionToken); err == nil {
		t.Fatal("a session JWT must not be accepted as a playback token")
	}

	// The reverse direction matters too: session validation must keep working
	// exactly as before, and must not accept a playback token either.
	if _, err := ValidateToken(sessionToken); err != nil {
		t.Fatalf("session token no longer validates: %v", err)
	}
	playback, _, err := IssuePlaybackToken(11, 42, DefaultPlaybackTokenTTL)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := ValidateToken(playback); err == nil {
		t.Error("a playback token must not be accepted as a session token")
	}
}

func TestPlaybackTokenExpiry(t *testing.T) {
	withTestSecret(t, "test-secret")

	key, err := PlaybackSigningKey()
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	claims := &PlaybackClaims{Purpose: PlaybackTokenPurpose, ResourceID: 1, UserID: 2}
	claims.ID = "jti-expired"
	claims.Issuer = PlaybackTokenIssuer
	claims.Audience = jwt.ClaimStrings{PlaybackTokenAudience}
	claims.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Minute))
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second))

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := ParsePlaybackToken(signed); !errors.Is(err, ErrPlaybackTokenExpired) {
		t.Errorf("error = %v, want ErrPlaybackTokenExpired", err)
	}
}

func TestPlaybackTokenRejectsForeignKeyAndGarbage(t *testing.T) {
	withTestSecret(t, "test-secret")

	// A token signed with a different deployment secret.
	withTestSecret(t, "other-secret")
	foreign, _, err := IssuePlaybackToken(1, 2, time.Minute)
	if err != nil {
		t.Fatalf("issue foreign: %v", err)
	}
	withTestSecret(t, "test-secret")
	if _, err := ParsePlaybackToken(foreign); err == nil {
		t.Error("a token signed with another secret was accepted")
	}

	for _, bad := range []string{"", "not.a.token", "a.b.c", strings.Repeat("x", 50)} {
		if _, err := ParsePlaybackToken(bad); err == nil {
			t.Errorf("garbage token %q was accepted", bad)
		}
	}
}

func TestPlaybackTokenRejectsNonHS256(t *testing.T) {
	withTestSecret(t, "test-secret")

	claims := &PlaybackClaims{Purpose: PlaybackTokenPurpose, ResourceID: 1, UserID: 2}
	claims.ID = "jti-none"
	claims.Issuer = PlaybackTokenIssuer
	claims.Audience = jwt.ClaimStrings{PlaybackTokenAudience}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Minute))

	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Skipf("this jwt version refuses alg=none signing: %v", err)
	}
	if _, err := ParsePlaybackToken(unsigned); err == nil {
		t.Error("an alg=none token was accepted")
	}
}

func TestPlaybackSigningKeyIsDerived(t *testing.T) {
	withTestSecret(t, "test-secret")

	key, err := PlaybackSigningKey()
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("derived key length = %d, want 32", len(key))
	}
	if string(key) == "test-secret" {
		t.Fatal("the playback key must be derived, not the raw JWT secret")
	}
}

func TestPlaybackSigningKeyNeedsConfig(t *testing.T) {
	previous := config.AppConfig
	t.Cleanup(func() { config.AppConfig = previous })
	config.AppConfig = nil

	if _, err := PlaybackSigningKey(); err == nil {
		t.Error("a missing config must be reported, not panic")
	}
	if _, _, err := IssuePlaybackToken(1, 2, time.Minute); err == nil {
		t.Error("issuing without config must fail")
	}
	if _, err := ParsePlaybackToken("whatever"); err == nil {
		t.Error("parsing without config must fail closed")
	}
}

func TestPlaybackTokenRequiresBindings(t *testing.T) {
	withTestSecret(t, "test-secret")

	if _, _, err := IssuePlaybackToken(0, 2, time.Minute); err == nil {
		t.Error("a grant without a user must be refused")
	}
	if _, _, err := IssuePlaybackToken(1, 0, time.Minute); err == nil {
		t.Error("a grant without a resource must be refused")
	}
	// Over-long TTLs are clamped to the short default.
	_, expiresAt, err := IssuePlaybackToken(1, 2, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if time.Until(expiresAt) > DefaultPlaybackTokenTTL+time.Second {
		t.Errorf("granted %s, want the %s cap", time.Until(expiresAt), DefaultPlaybackTokenTTL)
	}
}

// ----------------------------------------------------------------- helpers

// makeTestVideo renders a short clip with ffmpeg and returns its bytes.
func makeTestVideo(t *testing.T, size, codec string) []byte {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "source.mkv")

	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	if codec == "mjpeg" {
		args = append(args, "-f", "lavfi", "-i", "testsrc=size="+size+":rate=10:duration=1")
	} else {
		args = append(args, "-f", "lavfi", "-i",
			"testsrc=size="+size+":rate=10:duration=1", "-f", "lavfi", "-i",
			"sine=frequency=440:duration=1")
	}
	args = append(args, "-c:v", codec, "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", out)

	cmd := exec.Command(FFmpegBinary, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot synthesize a %s test clip with this ffmpeg build: %v (%s)", codec, err, output)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read test clip: %v", err)
	}
	return data
}

type videoProbe struct {
	Video struct {
		Codec    string `json:"codec_name"`
		PixFmt   string `json:"pix_fmt"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		HasAudio bool   `json:"-"`
	} `json:"video"`
	Audio struct {
		Codec string `json:"codec_name"`
	} `json:"audio"`
	Faststart bool `json:"-"`
}

// probeVideo inspects a normalized artifact with ffprobe.
func probeVideo(t *testing.T, path string) videoProbe {
	t.Helper()
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		path,
	).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}

	var raw struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			PixFmt    string `json:"pix_fmt"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("decode ffprobe output: %v", err)
	}

	var probe videoProbe
	for _, stream := range raw.Streams {
		switch stream.CodecType {
		case "video":
			probe.Video.Codec = stream.CodecName
			probe.Video.PixFmt = stream.PixFmt
			probe.Video.Width = stream.Width
			probe.Video.Height = stream.Height
			probe.Video.HasAudio = true
		case "audio":
			probe.Audio.Codec = stream.CodecName
		}
	}
	if !strings.Contains(raw.Format.FormatName, "mp4") && !strings.Contains(raw.Format.FormatName, "mov") {
		t.Errorf("container = %q, want an MP4/MOV family container", raw.Format.FormatName)
	}

	// faststart == the moov atom precedes the mdat atom.
	header, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	probe.Faststart = topLevelAtomOrder(header)
	return probe
}

// topLevelAtomOrder reports whether "moov" comes before "mdat".
func topLevelAtomOrder(data []byte) bool {
	moov, mdat := -1, -1
	for offset := 0; offset+8 <= len(data); {
		size := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
		switch string(data[offset+4 : offset+8]) {
		case "moov":
			if moov == -1 {
				moov = offset
			}
		case "mdat":
			if mdat == -1 {
				mdat = offset
			}
		}
		if size <= 0 {
			break
		}
		offset += size
	}
	return moov != -1 && (mdat == -1 || moov < mdat)
}
