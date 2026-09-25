package studyresources

import (
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"studsphere/backend/internal/shared/config"
)

// Canonical study-resource types. Mock tests are NOT study resources: they are
// their own domain (internal/mocktests) because each one owns a nested
// question/option graph with a single correct answer.
const (
	TypePastQuestions  = "past-questions"
	TypeStudyNotes     = "study-notes"
	TypeModelQuestions = "model-questions"
	TypeSyllabus       = "syllabus"
	TypeVideoLectures  = "video-lectures"
)

const defaultMaxVideoSizeMB = 200

// ErrMockTestNotAStudyResource is returned when a caller tries to store a mock
// test as a study resource.
var ErrMockTestNotAStudyResource = errors.New("mock tests are managed separately and are not a valid study resource type")

// canonicalTypes is the set of accepted canonical slugs.
var canonicalTypes = map[string]struct{}{
	TypePastQuestions:  {},
	TypeStudyNotes:     {},
	TypeModelQuestions: {},
	TypeSyllabus:       {},
	TypeVideoLectures:  {},
}

// typeSeparators collapses spaces, tabs, underscores and repeated dashes so
// legacy stored values ("Past Questions", "study_notes", "PAST-QUESTIONS")
// normalize onto the canonical slug.
var typeSeparators = regexp.MustCompile(`[\s_\-]+`)

// legacyTypeAliases covers spellings that do not normalize onto a canonical
// slug on their own. Keys are already separator-normalized. An empty value
// means "recognized but explicitly rejected".
var legacyTypeAliases = map[string]string{
	"note":           TypeStudyNotes,
	"notes":          TypeStudyNotes,
	"study-note":     TypeStudyNotes,
	"past-question":  TypePastQuestions,
	"model-question": TypeModelQuestions,
	"syllabi":        TypeSyllabus,
	"syllabuses":     TypeSyllabus,
	"video":          TypeVideoLectures,
	"videos":         TypeVideoLectures,
	"lecture":        TypeVideoLectures,
	"video-lecture":  TypeVideoLectures,
	"mock":           "", // explicit rejection below
	"mocks":          "",
	"mock-test":      "",
	"mock-tests":     "",
	"mocktest":       "",
	"mocktests":      "",
	"mock-question":  "",
	"mock-questions": "",
}

// AllTypes returns the canonical study-resource types in display order.
func AllTypes() []string {
	return []string{TypePastQuestions, TypeStudyNotes, TypeModelQuestions, TypeSyllabus, TypeVideoLectures}
}

// NormalizeType maps any accepted spelling onto its canonical slug. Unknown
// values and mock tests are rejected.
func NormalizeType(value string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = typeSeparators.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "", errors.New("resource type is required")
	}
	if _, ok := canonicalTypes[slug]; ok {
		return slug, nil
	}
	if alias, ok := legacyTypeAliases[slug]; ok {
		if alias == "" {
			return "", ErrMockTestNotAStudyResource
		}
		return alias, nil
	}
	return "", fmt.Errorf("invalid resource type %q (allowed: %s)", value, strings.Join(AllTypes(), ", "))
}

// IsValidType reports whether value normalizes to a canonical study-resource
// type. Mock tests are never valid.
func IsValidType(value string) bool {
	_, err := NormalizeType(value)
	return err == nil
}

// IsVideoType reports whether the resource is a video lecture.
func IsVideoType(value string) bool {
	canonical, err := NormalizeType(value)
	return err == nil && canonical == TypeVideoLectures
}

// storedTypeCandidates lists every raw spelling a row may hold. Legacy rows
// were written by the old endpoints and store human-readable values such as
// "Past Questions", while the API now speaks canonical slugs.
func storedTypeCandidates() []string {
	candidates := make([]string, 0, len(canonicalTypes)+len(legacyTypeAliases)*2+8)
	for slug := range canonicalTypes {
		candidates = append(candidates, slug)
	}
	for key, canonical := range legacyTypeAliases {
		candidates = append(candidates, key)
		if canonical != "" {
			candidates = append(candidates, canonical)
		}
	}
	candidates = append(candidates,
		"Past Questions", "Past Question",
		"Study Notes", "Study Note", "Notes",
		"Model Questions", "Model Question",
		"Syllabus", "Syllabi",
		"Video Lectures", "Video Lecture", "Video",
	)
	return candidates
}

// MatchingStoredTypes returns every stored spelling that normalizes to the
// given type, so filtering by the canonical slug still matches legacy rows.
// An unknown value returns nil and the caller keeps exact matching.
func MatchingStoredTypes(value string) []string {
	canonical, err := NormalizeType(value)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	matches := make([]string, 0, 4)
	for _, candidate := range storedTypeCandidates() {
		if seen[candidate] {
			continue
		}
		if got, err := NormalizeType(candidate); err == nil && got == canonical {
			seen[candidate] = true
			matches = append(matches, candidate)
		}
	}
	// Deterministic ordering keeps queries and tests stable.
	sort.Strings(matches)
	return matches
}

// Upload policy per resource type. The four document/image types keep their
// historical extension whitelist and 20MB cap; video lectures accept a set of
// common source containers with a larger, configurable cap and are normalized
// to a playable MP4 before storage.
type uploadPolicy struct {
	maxSize int64
	exts    map[string]bool
	// mimeByExt maps a validated extension to the content type of the SOURCE.
	// For documents it is empty: document uploads keep trusting the client
	// header exactly as before.
	mimeByExt map[string]string
	// normalized marks a policy whose uploads must be transcoded before storage.
	normalized bool
	// storeAsType is the content type persisted for a normalized upload.
	storeAsType string
}

// documentExtensions is the historical study-resource whitelist, unchanged.
var documentExtensions = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".ppt": true, ".pptx": true,
	".xls": true, ".xlsx": true, ".txt": true, ".csv": true,
	".zip": true, ".rar": true, ".7z": true,
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

// videoSourceExtensions are the containers an admin may upload for a video
// lecture. They are transcoded to a broadly playable H.264/AAC MP4 on upload
// (see shared/utils.NormalizeVideoToMP4), so the source container and its
// codecs never reach playback.
var videoSourceExtensions = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/x-m4v",
	".mov":  "video/quicktime",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".webm": "video/webm",
}

// NormalizedVideoContentType / Extension describe the artifact we actually
// store. The client-supplied content type is never trusted for video.
const (
	NormalizedVideoContentType = "video/mp4"
	NormalizedVideoExtension   = ".mp4"
)

// DocumentExtensions exposes the document/image whitelist (read-only copy).
func DocumentExtensions() map[string]bool {
	out := make(map[string]bool, len(documentExtensions))
	for ext := range documentExtensions {
		out[ext] = true
	}
	return out
}

// VideoSourceExtensions exposes the accepted video upload extensions.
func VideoSourceExtensions() []string {
	exts := make([]string, 0, len(videoSourceExtensions))
	for ext := range videoSourceExtensions {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

// UploadPolicyForType returns the upload policy that applies to a canonical
// resource type. Unknown types fall back to the document policy so legacy rows
// with a non-canonical type keep uploading exactly as before.
func UploadPolicyForType(resourceType string) uploadPolicy {
	if IsVideoType(resourceType) {
		return uploadPolicy{
			maxSize: MaxVideoSizeBytes(),
			exts:    extSet(videoSourceExtensions),
			// The stored MIME is always the normalized one; the map is only
			// used for validation diagnostics and temp-file naming.
			mimeByExt:   videoSourceExtensions,
			normalized:  true,
			storeAsType: NormalizedVideoContentType,
		}
	}
	return uploadPolicy{maxSize: maxFileSize, exts: documentExtensions}
}

// RequiresNormalization reports whether an upload for this resource type must be
// transcoded to a playable MP4 before it is stored.
func RequiresNormalization(resourceType string) bool {
	return UploadPolicyForType(resourceType).normalized
}

func extSet(m map[string]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for ext := range m {
		out[ext] = true
	}
	return out
}

// MaxVideoSizeBytes returns the configured video upload cap, defaulting to
// 200MB when the config helper is unavailable (e.g. in tests).
func MaxVideoSizeBytes() int64 {
	if config.AppConfig != nil && config.AppConfig.StudyResourceVideoMaxSizeMB > 0 {
		return int64(config.AppConfig.StudyResourceVideoMaxSizeMB) * 1024 * 1024
	}
	return int64(defaultMaxVideoSizeMB) * 1024 * 1024
}

// maxSizeLabel renders a human readable cap for error messages.
func maxSizeLabel(bytes int64) string {
	return fmt.Sprintf("%dMB", bytes/(1024*1024))
}

// validateUploadForType validates an upload against the policy of the given
// resource type. It returns the normalized lowercase extension, the content
// type derived from that extension (never the client header) and whether the
// upload must be transcoded before storage.
func validateUploadForType(fileHeader *multipart.FileHeader, resourceType string) (ext string, contentType string, normalized bool, err error) {
	policy := UploadPolicyForType(resourceType)
	ext = strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == "" {
		return "", "", false, errors.New("file has no extension")
	}
	if !policy.exts[ext] {
		if IsVideoType(resourceType) {
			return "", "", false, fmt.Errorf("video lectures must be uploaded as %s", strings.Join(VideoSourceExtensions(), ", "))
		}
		return "", "", false, errors.New("file type not allowed")
	}
	if fileHeader.Size > policy.maxSize {
		return "", "", false, fmt.Errorf("file size exceeds limit of %s", maxSizeLabel(policy.maxSize))
	}
	if mime, ok := policy.mimeByExt[ext]; ok {
		return ext, mime, policy.normalized, nil
	}
	return ext, "", false, nil
}

// StoredFileSatisfiesType reports whether an ALREADY stored file would be
// accepted by the upload policy of resourceType. fileName is the original
// upload name and objectKey the storage key; the first one that carries an
// extension decides. A row with no extension at all (a legacy row that lost
// its file name) cannot be validated and is reported as satisfied so a
// legitimate type change is never blocked by missing metadata.
func StoredFileSatisfiesType(fileName, objectKey, resourceType string) bool {
	policy := UploadPolicyForType(resourceType)
	for _, candidate := range []string{fileName, objectKey} {
		ext := strings.ToLower(filepath.Ext(strings.TrimSpace(candidate)))
		if ext == "" {
			continue
		}
		return policy.exts[ext]
	}
	return true
}

// ValidateTypeChange rejects a type change whose stored file no longer
// satisfies the target type's upload policy (e.g. turning a stored .pdf row
// into a video lecture, or a .mp4 row into a document category).
func ValidateTypeChange(fileName, objectKey, resourceType string) error {
	if StoredFileSatisfiesType(fileName, objectKey, resourceType) {
		return nil
	}

	name := strings.TrimSpace(fileName)
	if name == "" {
		name = strings.TrimSpace(objectKey)
	}
	if IsVideoType(resourceType) {
		return fmt.Errorf("cannot change type to %q: the stored file %q is not a valid video lecture (allowed: %s)",
			resourceType, name, strings.Join(VideoSourceExtensions(), ", "))
	}
	return fmt.Errorf("cannot change type to %q: the stored file %q is not allowed for this category", resourceType, name)
}
