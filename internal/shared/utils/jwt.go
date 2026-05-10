package utils

import (
	"errors"
	"time"

	"studsphere/backend/internal/shared/config"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID     uint   `json:"user_id"`
	ProviderID uint   `json:"provider_id,omitempty"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
	jwt.RegisteredClaims
}

type TokenOptions struct {
	UserID     uint
	Email      string
	Role       string
	ProviderID uint
	FirstName  string
	LastName   string
	ImageURL   string
}

func GenerateToken(userID uint, email, role string, providerID uint) (string, error) {
	return GenerateTokenWithClaims(TokenOptions{
		UserID:     userID,
		Email:      email,
		Role:       role,
		ProviderID: providerID,
	})
}

func GenerateTokenWithClaims(opts TokenOptions) (string, error) {
	expiryDuration, err := time.ParseDuration(config.AppConfig.JWTExpiry)
	if err != nil {
		expiryDuration = 24 * time.Hour
	}

	claims := &Claims{
		UserID:     opts.UserID,
		ProviderID: opts.ProviderID,
		Email:      opts.Email,
		Role:       opts.Role,
		FirstName:  opts.FirstName,
		LastName:   opts.LastName,
		ImageURL:   opts.ImageURL,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiryDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}

func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.AppConfig.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
