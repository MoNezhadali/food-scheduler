package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type Claims struct {
	UserID string    `json:"user_id"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	Type   TokenType `json:"type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds until access token expires
}

// Service is the port that application use-cases depend on for token operations.
type Service interface {
	IssueTokens(userID, email, role string) (TokenPair, error)
	Validate(token string, expectedType TokenType) (*Claims, error)
}

type JWTService struct {
	secret          []byte
	accessDuration  time.Duration
	refreshDuration time.Duration
}

func NewJWTService(secret string) *JWTService {
	return &JWTService{
		secret:          []byte(secret),
		accessDuration:  15 * time.Minute,
		refreshDuration: 7 * 24 * time.Hour,
	}
}

func (s *JWTService) IssueTokens(userID, email, role string) (TokenPair, error) {
	access, err := s.sign(userID, email, role, TokenTypeAccess, s.accessDuration)
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign access token: %w", err)
	}
	refresh, err := s.sign(userID, email, role, TokenTypeRefresh, s.refreshDuration)
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign refresh token: %w", err)
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int(s.accessDuration.Seconds()),
	}, nil
}

func (s *JWTService) Validate(tokenStr string, expectedType TokenType) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	if claims.Type != expectedType {
		return nil, fmt.Errorf("expected %q token, got %q", expectedType, claims.Type)
	}
	return claims, nil
}

func (s *JWTService) sign(userID, email, role string, t TokenType, dur time.Duration) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		Type:   t,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(dur)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}
