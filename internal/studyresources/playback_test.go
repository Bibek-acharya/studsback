package studyresources

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"
)

// playbackRouter wires the REAL registered routes behind a fake auth middleware
// that behaves like the production middleware.Auth: a request WITHOUT a valid
// session is aborted with 401.
//
// An empty session therefore proves a route is reachable without a session —
// exactly the situation a <video> element is in, since it can only request a
// URL and cannot attach an Authorization header.
func playbackRouter(t *testing.T, db *gorm.DB, session string) *gin.Engine {
	t.Helper()
	withTestJWTSecret(t)
	gin.SetMode(gin.TestMode)

	r := gin.New()
	authMW := func(c *gin.Context) {
		if session == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
			return
		}
		c.Set("user_id", uint(11))
		c.Set("user_role", "student")
		c.Next()
	}
	RegisterRoutes(r, authMW, nil, NewHandler(NewService(NewRepository(db))))
	return r
}

func playbackFixture(t *testing.T) (*gorm.DB, *StudyResource) {
	t.Helper()
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	video := &StudyResource{
		Title:           "Kinematics lecture",
		ResourceType:    TypeVideoLectures,
		FileName:        "kinematics.mp4",
		FilePath:        "private/study-resources/kinematics.mp4",
		FileURL:         "/uploads/private/study-resources/kinematics.mp4",
		FileSize:        4096,
		MimeType:        NormalizedVideoContentType,
		DurationSeconds: 900,
		IsPublished:     true,
	}
	draft := &StudyResource{
		Title:        "Draft lecture",
		ResourceType: TypeVideoLectures,
		FileName:     "draft.mp4",
		FilePath:     "private/study-resources/draft.mp4",
		FileURL:      "/uploads/private/study-resources/draft.mp4",
		FileSize:     2048,
		MimeType:     NormalizedVideoContentType,
		IsPublished:  false,
	}
	doc := &StudyResource{
		Title:        "Syllabus",
		ResourceType: TypeSyllabus,
		FileName:     "syllabus.pdf",
		FilePath:     "study-resources/syllabus.pdf",
		FileURL:      "/uploads/study-resources/syllabus.pdf",
		FileSize:     1024,
		MimeType:     "application/pdf",
		IsPublished:  true,
	}
	for _, resource := range []*StudyResource{video, draft, doc} {
		if err := NewRepository(db).CreateResource(resource); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return db, video
}

type playbackTokenEnvelope struct {
	Data struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
		StreamURL string `json:"stream_url"`
	} `json:"data"`
}

// testPlaybackClaims builds a claim set with full control over every binding.
// Promoted fields cannot appear in composite literals with the module's Go
// version, so they are assigned explicitly.
func testPlaybackClaims(purpose, audience, issuer, jti string, resourceID, userID uint, expiresAt time.Time) *utils.PlaybackClaims {
	claims := &utils.PlaybackClaims{Purpose: purpose, ResourceID: resourceID, UserID: userID}
	claims.ID = jti
	claims.Issuer = issuer
	if audience != "" {
		claims.Audience = jwt.ClaimStrings{audience}
	}
	claims.ExpiresAt = jwt.NewNumericDate(expiresAt)
	return claims
}

// mustSessionToken mints a NORMAL session JWT, which must never be usable as a
// playback token.
func mustSessionToken(t *testing.T) string {
	t.Helper()
	token, err := utils.GenerateToken(11, "student@example.com", "student", 0)
	if err != nil {
		t.Fatalf("generate session token: %v", err)
	}
	return token
}

// mustExpiredPlaybackToken signs a correctly-scoped playback token whose expiry
// is already in the past.
func mustExpiredPlaybackToken(t *testing.T, resourceID uint) string {
	t.Helper()
	key, err := utils.PlaybackSigningKey()
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	claims := testPlaybackClaims(
		utils.PlaybackTokenPurpose, utils.PlaybackTokenAudience, utils.PlaybackTokenIssuer,
		"jti-expired", resourceID, 11, time.Now().Add(-time.Minute),
	)
	claims.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Minute))
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}
	return signed
}

// storageClientSwap installs a storage client for the duration of a test and
// returns the previous one so the caller can restore it.
func storageClientSwap(t *testing.T, client *minio.Client) *minio.Client {
	t.Helper()
	previous := storage.Client
	storage.Client = client
	t.Cleanup(func() { storage.Client = previous })
	return previous
}

func requestPlaybackToken(t *testing.T, r *gin.Engine, id uint) (*httptest.ResponseRecorder, playbackTokenEnvelope) {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/v1/study-resources/"+itoaResource(id)+"/playback-token", nil))
	var envelope playbackTokenEnvelope
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode token response: %v (body %s)", err, rec.Body.String())
		}
	}
	return rec, envelope
}

// The token endpoint is behind the session Auth middleware.
func TestPlaybackTokenRequiresAuthentication(t *testing.T) {
	db, video := playbackFixture(t)
	anonymous := playbackRouter(t, db, "")

	rec, _ := requestPlaybackToken(t, anonymous, video.ID)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous playback-token = %d, want 401 (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), video.FilePath) {
		t.Errorf("401 body leaked the object key: %s", rec.Body.String())
	}
}

func TestPlaybackTokenIssuesResourceBoundGrant(t *testing.T) {
	db, video := playbackFixture(t)
	r := playbackRouter(t, db, "session")

	rec, envelope := requestPlaybackToken(t, r, video.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("playback-token = %d (body %s)", rec.Code, rec.Body.String())
	}
	if envelope.Data.Token == "" {
		t.Fatal("no token returned")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("cache-control = %q, want no-store", got)
	}

	// The response must never leak the object key.
	if strings.Contains(rec.Body.String(), "private/study-resources") || strings.Contains(rec.Body.String(), video.FilePath) {
		t.Errorf("playback-token response leaked the object key: %s", rec.Body.String())
	}
	// No presigned/blob URL is handed out: the stream URL points at our own API.
	if !strings.HasPrefix(envelope.Data.StreamURL, "/api/v1/study-resources/") {
		t.Errorf("stream_url = %q, want an API-relative stream path", envelope.Data.StreamURL)
	}
	if strings.Contains(envelope.Data.StreamURL, "X-Amz-Signature") || strings.Contains(envelope.Data.StreamURL, "blob:") {
		t.Errorf("stream_url must not be a presigned/blob URL: %q", envelope.Data.StreamURL)
	}

	// expires_at is ~5 minutes out.
	expiresAt, err := time.Parse(time.RFC3339, envelope.Data.ExpiresAt)
	if err != nil {
		t.Fatalf("parse expires_at %q: %v", envelope.Data.ExpiresAt, err)
	}
	if remaining := time.Until(expiresAt); remaining <= 0 || remaining > utils.DefaultPlaybackTokenTTL+time.Minute {
		t.Errorf("expires_at in %s, want a short positive TTL", remaining)
	}

	// The token is bound to this resource for this user.
	claims, err := utils.ParsePlaybackToken(envelope.Data.Token)
	if err != nil {
		t.Fatalf("parse returned token: %v", err)
	}
	if claims.ResourceID != video.ID {
		t.Errorf("resource_id = %d, want %d", claims.ResourceID, video.ID)
	}
	if claims.UserID != 11 {
		t.Errorf("user_id = %d, want the authenticated user 11", claims.UserID)
	}
	if claims.Purpose != utils.PlaybackTokenPurpose || claims.Scope() != utils.PlaybackTokenAudience {
		t.Errorf("purpose/audience = %q/%q", claims.Purpose, claims.Audience)
	}
	if claims.ID == "" {
		t.Error("jti is empty")
	}

	// The stream_url carries the token as ?pt=.
	parsed, err := url.Parse(envelope.Data.StreamURL)
	if err != nil {
		t.Fatalf("parse stream_url: %v", err)
	}
	if got := parsed.Query().Get(playbackTokenQueryParam); got != envelope.Data.Token {
		t.Errorf("stream_url pt = %q, want the issued token", got)
	}
}

func TestPlaybackTokenRejectsNonPlayableResources(t *testing.T) {
	db, _ := playbackFixture(t)
	r := playbackRouter(t, db, "session")

	var draft, doc *StudyResource
	if err := db.First(&draft, "title = ?", "Draft lecture").Error; err != nil {
		t.Fatalf("load draft: %v", err)
	}
	if err := db.First(&doc, "title = ?", "Syllabus").Error; err != nil {
		t.Fatalf("load doc: %v", err)
	}

	for _, tc := range []struct {
		name       string
		id         uint
		wantStatus int
	}{
		{name: "draft video", id: draft.ID, wantStatus: http.StatusNotFound},
		{name: "document", id: doc.ID, wantStatus: http.StatusBadRequest},
		{name: "unknown id", id: 987654, wantStatus: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, _ := requestPlaybackToken(t, r, tc.id)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// No video bytes without a valid, matching playback token. Every case runs on
// a session-less router, i.e. the situation a <video> element is in.
func TestStreamRequiresPlaybackToken(t *testing.T) {
	withTestJWTSecret(t)
	db, video := playbackFixture(t)

	sessionToken := mustSessionToken(t)
	expired := mustExpiredPlaybackToken(t, video.ID)
	otherResource := playbackTokenFor(t, 11, video.ID+999)

	// Object storage is unavailable in unit tests: a request that passed the
	// gate would fail later with 404 on the object read, so a 401/403 here
	// proves the gate rejected it BEFORE storage.
	previous := storageClientSwap(t, nil)
	defer storageClientSwap(t, previous)

	cases := []struct {
		name       string
		session    string
		query      string
		wantStatus int
	}{
		{name: "anonymous, no token", query: "", wantStatus: http.StatusUnauthorized},
		{name: "anonymous, session JWT as pt", query: "?pt=" + sessionToken, wantStatus: http.StatusUnauthorized},
		{name: "anonymous, legacy session query param", query: "?token=" + sessionToken, wantStatus: http.StatusUnauthorized},
		{name: "anonymous, expired playback token", query: "?pt=" + expired, wantStatus: http.StatusUnauthorized},
		{name: "anonymous, garbage token", query: "?pt=not-a-token", wantStatus: http.StatusUnauthorized},
		{name: "anonymous, token bound to another resource", query: "?pt=" + otherResource, wantStatus: http.StatusForbidden},
		{name: "logged in, no playback token", session: "session", query: "", wantStatus: http.StatusUnauthorized},
		{name: "logged in, session JWT as pt", session: "session", query: "?pt=" + sessionToken, wantStatus: http.StatusUnauthorized},
		{name: "logged in, token bound to another resource", session: "session", query: "?pt=" + otherResource, wantStatus: http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := playbackRouter(t, db, tc.session)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
				"/api/v1/study-resources/"+itoaResource(video.ID)+"/stream"+tc.query, nil))
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if body := rec.Body.String(); body != "" && !strings.Contains(body, `"success":false`) {
				t.Errorf("unexpected body: %s", body)
			}
		})
	}
}

// A <video> element cannot send an Authorization header, so the stream route
// must be reachable WITHOUT a session: the playback token alone has to get the
// request to the handler and through its gate.
func TestStreamRouteIsNotSessionGated(t *testing.T) {
	db, video := playbackFixture(t)
	// This router's auth middleware rejects every session-less request, exactly
	// like middleware.Auth. A stream route wrapped in it would answer 401.
	r := playbackRouter(t, db, "")

	token := playbackTokenFor(t, 11, video.ID)
	previous := storageClientSwap(t, nil)
	defer storageClientSwap(t, previous)

	// Document the scenario: no Authorization header, no session cookie.
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/study-resources/"+itoaResource(video.ID)+"/stream?pt="+url.QueryEscape(token), nil)
	if req.Header.Get("Authorization") != "" {
		t.Fatal("test setup: the stream request must not carry an Authorization header")
	}
	if _, err := req.Cookie("token"); err == nil {
		t.Fatal("test setup: the stream request must not carry a session cookie")
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Reaching the handler at all proves the middleware let it through: the
	// gate accepts the token, so the request only fails on the (unavailable)
	// object read.
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("a valid playback token was rejected before the handler: %d (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Authentication required") {
		t.Fatalf("the session middleware still gates the stream route: %s", rec.Body.String())
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 from the unavailable object store", rec.Code)
	}

	// The token endpoint, in contrast, must stay session-gated.
	tokenRec := httptest.NewRecorder()
	r.ServeHTTP(tokenRec, httptest.NewRequest(http.MethodGet,
		"/api/v1/study-resources/"+itoaResource(video.ID)+"/playback-token", nil))
	if tokenRec.Code != http.StatusUnauthorized {
		t.Errorf("playback-token without a session = %d, want 401 (body %s)", tokenRec.Code, tokenRec.Body.String())
	}
}

// A non-video or draft row must not yield bytes even with a valid token, on the
// session-less (media element) route.
func TestStreamRouteRejectsNonVideoAndDraftWithoutSession(t *testing.T) {
	db, video := playbackFixture(t)
	r := playbackRouter(t, db, "")

	var draft, doc *StudyResource
	if err := db.First(&draft, "title = ?", "Draft lecture").Error; err != nil {
		t.Fatalf("load draft: %v", err)
	}
	if err := db.First(&doc, "title = ?", "Syllabus").Error; err != nil {
		t.Fatalf("load doc: %v", err)
	}

	previous := storageClientSwap(t, nil)
	defer storageClientSwap(t, previous)

	for _, tc := range []struct {
		name       string
		id         uint
		wantStatus int
	}{
		{name: "document", id: doc.ID, wantStatus: http.StatusBadRequest},
		{name: "draft video", id: draft.ID, wantStatus: http.StatusNotFound},
		{name: "published video still passes the gate", id: video.ID, wantStatus: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
				"/api/v1/study-resources/"+itoaResource(tc.id)+"/stream?pt="+
					url.QueryEscape(playbackTokenFor(t, 11, tc.id)), nil))
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if body := rec.Body.String(); body != "" && !strings.Contains(body, `"success":false`) {
				t.Errorf("unexpected body: %s", body)
			}
		})
	}
}

// A valid token gets past the gate: the request then fails only because object
// storage is unavailable in the unit test environment.
func TestStreamWithValidTokenPassesTheGate(t *testing.T) {
	db, video := playbackFixture(t)
	r := playbackRouter(t, db, "session")

	token := playbackTokenFor(t, 11, video.ID)
	previous := storageClientSwap(t, nil)
	defer storageClientSwap(t, previous)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/v1/study-resources/"+itoaResource(video.ID)+"/stream?pt="+url.QueryEscape(token), nil))

	// Past the gate = no 401/403; the object read itself cannot succeed here.
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("valid token was rejected: %d (body %s)", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 from the unavailable object store", rec.Code)
	}
}

// The security headers must be on the bytes we do serve.
func TestStreamSetsCacheAndSecurityHeaders(t *testing.T) {
	withTestJWTSecret(t)
	h, _, resource, body := streamFixture(t)
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/1/stream", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.streamVideo(c, resource, bytes.NewReader(body), int64(len(body)))

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("x-content-type-options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "private") {
		t.Errorf("cache-control = %q, want a private directive", got)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("cache-control = %q, want immutable", got)
	}
}

// ------------------------------------------------------- token helper checks

func TestPlaybackTokenRejectsSessionJWT(t *testing.T) {
	withTestJWTSecret(t)
	if _, err := utils.ParsePlaybackToken(mustSessionToken(t)); err == nil {
		t.Fatal("a session JWT must not parse as a playback token")
	}
}

func TestPlaybackTokenRejectsOtherPurposes(t *testing.T) {
	withTestJWTSecret(t)
	key, err := utils.PlaybackSigningKey()
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}

	minute := time.Now().Add(time.Minute)
	cases := []struct {
		name   string
		claims *utils.PlaybackClaims
	}{
		{
			name:   "wrong purpose",
			claims: testPlaybackClaims("something-else", utils.PlaybackTokenAudience, utils.PlaybackTokenIssuer, "jti-1", 5, 9, minute),
		},
		{
			name:   "wrong audience",
			claims: testPlaybackClaims(utils.PlaybackTokenPurpose, "somewhere-else", utils.PlaybackTokenIssuer, "jti-2", 5, 9, minute),
		},
		{
			name:   "no audience at all",
			claims: testPlaybackClaims(utils.PlaybackTokenPurpose, "", utils.PlaybackTokenIssuer, "jti-2b", 5, 9, minute),
		},
		{
			name:   "wrong issuer",
			claims: testPlaybackClaims(utils.PlaybackTokenPurpose, utils.PlaybackTokenAudience, "somebody-else", "jti-3", 5, 9, minute),
		},
		{
			name:   "missing resource binding",
			claims: testPlaybackClaims(utils.PlaybackTokenPurpose, utils.PlaybackTokenAudience, utils.PlaybackTokenIssuer, "jti-4", 0, 9, minute),
		},
		{
			name:   "missing user binding",
			claims: testPlaybackClaims(utils.PlaybackTokenPurpose, utils.PlaybackTokenAudience, utils.PlaybackTokenIssuer, "jti-5", 5, 0, minute),
		},
		{
			name:   "missing jti",
			claims: testPlaybackClaims(utils.PlaybackTokenPurpose, utils.PlaybackTokenAudience, utils.PlaybackTokenIssuer, "", 5, 9, minute),
		},
		{
			name:   "no expiry",
			claims: testPlaybackClaims(utils.PlaybackTokenPurpose, utils.PlaybackTokenAudience, utils.PlaybackTokenIssuer, "jti-6", 5, 9, time.Time{}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, tc.claims).SignedString(key)
			if err != nil {
				t.Fatalf("sign: %v", err)
			}
			if _, err := utils.ParsePlaybackToken(signed); err == nil {
				t.Fatal("token with an invalid binding was accepted")
			}
		})
	}
}

func TestPlaybackTokenRejectsNonHS256(t *testing.T) {
	withTestJWTSecret(t)
	claims := testPlaybackClaims(utils.PlaybackTokenPurpose, utils.PlaybackTokenAudience, utils.PlaybackTokenIssuer,
		"jti-none", 1, 2, time.Now().Add(time.Minute))
	// "none" and other algorithms must never be accepted, even with a
	// perfectly formed claim set.
	signed, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Skipf("cannot mint an alg=none token with this library version: %v", err)
	}
	if _, err := utils.ParsePlaybackToken(signed); err == nil {
		t.Fatal("an alg=none token was accepted")
	}

	// A token signed with the RAW session secret must be rejected too.
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.AppConfig.JWTSecret))
	if err != nil {
		t.Fatalf("sign with raw secret: %v", err)
	}
	if _, err := utils.ParsePlaybackToken(raw); err == nil {
		t.Fatal("a token signed with the raw session secret was accepted")
	}
}

func TestPlaybackTokenTTLIsCapped(t *testing.T) {
	withTestJWTSecret(t)
	if _, err := utils.ParsePlaybackToken(""); err == nil {
		t.Error("an empty token must be rejected")
	}
	// A caller asking for a day-long grant is clamped to the short default.
	_, expiresAt, err := utils.IssuePlaybackToken(1, 2, 24*time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if remaining := time.Until(expiresAt); remaining > utils.DefaultPlaybackTokenTTL+time.Second {
		t.Errorf("requested 24h, granted %s: playback grants must stay short", remaining)
	}
	// A zero/negative TTL falls back to the default rather than expiring now.
	_, fallback, err := utils.IssuePlaybackToken(1, 2, 0)
	if err != nil {
		t.Fatalf("issue fallback: %v", err)
	}
	if time.Until(fallback) <= 0 {
		t.Error("a zero TTL must fall back to the default, not expire immediately")
	}
	// Missing bindings are refused.
	if _, _, err := utils.IssuePlaybackToken(0, 2, time.Minute); err == nil {
		t.Error("a token without a user must be refused")
	}
	if _, _, err := utils.IssuePlaybackToken(1, 0, time.Minute); err == nil {
		t.Error("a token without a resource must be refused")
	}
}

func TestPlaybackSigningKeyIsDistinctFromSessionSecret(t *testing.T) {
	previous := config.AppConfig
	t.Cleanup(func() { config.AppConfig = previous })
	config.AppConfig = &config.Config{JWTSecret: "shared-secret"}

	key, err := utils.PlaybackSigningKey()
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	if string(key) == "shared-secret" {
		t.Fatal("playback key must not be the raw JWT secret")
	}
	// Stable across calls within the same deployment.
	again, err := utils.PlaybackSigningKey()
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	if string(again) != string(key) {
		t.Error("playback key must be deterministic")
	}
}
