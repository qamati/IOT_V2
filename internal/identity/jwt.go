package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

type userContextKey string

const userCtxKey userContextKey = "iotp.user"

// Claims defines the JWT token payload.
type Claims struct {
	UserID    ids.ID          `json:"sub"`
	TenantID  string          `json:"tenant_id,omitempty"`
	Authority model.Authority `json:"authority"`
	Email     string          `json:"email"`
	jwt.RegisteredClaims
}

// GenerateToken generates an HMAC-SHA256 signed JWT token for a user.
func GenerateToken(user *model.User, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now()
	tenantStr := ""
	if user.TenantID != nil {
		tenantStr = user.TenantID.String()
	}

	claims := Claims{
		UserID:    user.ID,
		TenantID:  tenantStr,
		Authority: user.Authority,
		Email:     user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "iotp",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("identity: failed to sign JWT token: %w", err)
	}

	return signed, nil
}

// ValidateToken parses and validates a JWT token string.
func ValidateToken(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// WithUser binds the validated Claims to context.
func WithUser(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, userCtxKey, claims)
}

// UserFromContext retrieves the Claims from context.
func UserFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(userCtxKey).(*Claims)
	return claims, ok
}
