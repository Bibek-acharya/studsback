package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"studsphere/backend/internal/shared/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Purpose-scoped playback tokens.
//
// Video bytes are only served to a signed-in user through a token that is
// bound to ONE resource and expires in minutes. This is deliberately NOT the
// normal session JWT:
//   - it is signed with a key DERIVED from the JWT secret, so a session token
//     never validates as a playback token even though it carries a valid
//     signature for the same deployment;
//   - it carries a dedicated purpose and audience, so a token minted for any
//     other purpose is rejected;
//   - it is bound to a resource id, a user id and a unique jti, with a short
//     expiry;
//   - only HS256 is accepted.
//
// Session-token semantics (GenerateToken / ValidateToken) are untouched.

const (
	// PlaybackTokenPurpose marks a token as a media playback grant.
	PlaybackTokenPurpose = "study-resource-playback"
	// PlaybackTokenAudience scopes the token to the video stream endpoint.
	PlaybackTokenAudience = "study-resource-stream"
	// PlaybackTokenIssuer identifies this API as the minting authority.
	PlaybackTokenIssuer = "studsphere-api"
	// DefaultPlaybackTokenTTL is short by design: a playback grant is not a
	// session.
	DefaultPlaybackTokenTTL = 5 * time.Minute
	// playbackKeyContext domain-separates the derived signing key.
	playbackKeyContext = "studsphere/playback-token/v1"
)

var (
	// ErrPlaybackTokenInvalid covers every rejection reason; callers must not
	// distinguish them in their response.
	ErrPlaybackTokenInvalid = errors.New("invalid playback token")
	// ErrPlaybackTokenExpired is returned for an otherwise valid but expired
	// token.
	ErrPlaybackTokenExpired = errors.New("playback token expired")
)

// PlaybackClaims is the payload of a playback token.
//
// The audience is the embedded jwt.RegisteredClaims.Audience: declaring a second
// `aud` field here would collide with the embedded one (same JSON tag), and Go's
// encoder drops both, silently producing an unscoped token.
type PlaybackClaims struct {
	// Purpose must equal PlaybackTokenPurpose.
	Purpose string `json:"purpose"`
	// ResourceID is the single study resource this grant may stream.
	ResourceID uint `json:"resource_id"`
	// UserID is the authenticated user the grant was minted for.
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// Scope returns the token audience, or "" when none is set.
func (c *PlaybackClaims) Scope() string {
	if c == nil || len(c.Audience) == 0 {
		return ""
	}
	return c.Audience[0]
}

// PlaybackSigningKey derives a dedicated HMAC key from the configured JWT
// secret. Deriving keeps playback tokens cryptographically distinct from
// session tokens without introducing another secret to configure.
func PlaybackSigningKey() ([]byte, error) {
	if config.AppConfig == nil {
		return nil, errors.New("configuration is not loaded")
	}
	mac := hmac.New(sha256.New, []byte(config.AppConfig.JWTSecret))
	if _, err := mac.Write([]byte(playbackKeyContext)); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

// IssuePlaybackToken mints a short-lived, resource-bound playback token. It
// returns the token and its expiry.
func IssuePlaybackToken(userID, resourceID uint, ttl time.Duration) (string, time.Time, error) {
	if userID == 0 || resourceID == 0 {
		return "", time.Time{}, errors.New("playback token requires a user and a resource")
	}
	if ttl <= 0 {
		ttl = DefaultPlaybackTokenTTL
	}
	if ttl > DefaultPlaybackTokenTTL {
		// Never mint a long-lived playback grant, whatever a caller asks for.
		ttl = DefaultPlaybackTokenTTL
	}
	key, err := PlaybackSigningKey()
	if err != nil {
		return "", time.Time{}, err
	}

	now := time.Now()
	expiresAt := now.Add(ttl)
	claims := &PlaybackClaims{
		Purpose:    PlaybackTokenPurpose,
		ResourceID: resourceID,
		UserID:     userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    PlaybackTokenIssuer,
			Audience:  jwt.ClaimStrings{PlaybackTokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign playback token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParsePlaybackToken validates a playback token and returns its claims. It
// rejects anything that is not an unexpired, HS256-signed token with the
// playback purpose, the stream audience, this issuer, a unique id, and both a
// user and a resource binding. A normal session token fails this.
func ParsePlaybackToken(tokenString string) (*PlaybackClaims, error) {
	if tokenString == "" {
		return nil, ErrPlaybackTokenInvalid
	}
	key, err := PlaybackSigningKey()
	if err != nil {
		return nil, ErrPlaybackTokenInvalid
	}

	claims := &PlaybackClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", token.Header["alg"])
		}
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing algorithm %s", token.Method.Alg())
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(PlaybackTokenIssuer),
		jwt.WithAudience(PlaybackTokenAudience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrPlaybackTokenExpired
		}
		return nil, ErrPlaybackTokenInvalid
	}
	if !token.Valid {
		return nil, ErrPlaybackTokenInvalid
	}

	switch {
	case claims.Purpose != PlaybackTokenPurpose:
		return nil, ErrPlaybackTokenInvalid
	case claims.Scope() != PlaybackTokenAudience:
		return nil, ErrPlaybackTokenInvalid
	case claims.Issuer != PlaybackTokenIssuer:
		return nil, ErrPlaybackTokenInvalid
	case claims.ID == "":
		return nil, ErrPlaybackTokenInvalid
	case claims.UserID == 0:
		return nil, ErrPlaybackTokenInvalid
	case claims.ResourceID == 0:
		return nil, ErrPlaybackTokenInvalid
	}
	return claims, nil
}
