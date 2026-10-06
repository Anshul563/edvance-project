package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// accessTokenAlgorithm is the only JWT signing algorithm we accept.
const accessTokenAlgorithm = "HS256"

// GenerateAccessToken issues a short-lived HS256 JWT for the given user.
// sessionID binds the token to its auth_sessions row; pass uuid.Nil to
// omit the sid claim. It returns the signed token and the generated JTI.
func GenerateAccessToken(
	userID uuid.UUID,
	sessionID uuid.UUID,
	secret string,
	issuer string,
	audience string,
	ttl time.Duration,
) (tokenString string, jti uuid.UUID, err error) {
	if secret == "" {
		return "", uuid.Nil, errors.New("token: access secret is required")
	}

	if issuer == "" {
		return "", uuid.Nil, errors.New("token: issuer is required")
	}

	if audience == "" {
		return "", uuid.Nil, errors.New("token: audience is required")
	}

	if ttl <= 0 {
		return "", uuid.Nil, errors.New("token: ttl must be positive")
	}

	now := time.Now()
	jti = uuid.New()

	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        jti.String(),
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	if sessionID != uuid.Nil {
		claims.SessionID = sessionID.String()
	}

	signed, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString([]byte(secret))
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("token: sign access token: %w", err)
	}

	return signed, jti, nil
}

// ValidateAccessToken verifies signature, expiry, issuer, audience and
// algorithm, and returns the claims. Only HS256 is accepted.
func ValidateAccessToken(
	tokenString string,
	secret string,
	issuer string,
	audience string,
) (*AccessClaims, error) {
	if tokenString == "" {
		return nil, errors.New("token: token is required")
	}

	if secret == "" {
		return nil, errors.New("token: access secret is required")
	}

	claims := &AccessClaims{}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{accessTokenAlgorithm}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
	)

	parsed, err := parser.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			// Enforce HMAC before using the secret (algorithm confusion
			// defense); WithValidMethods already rejects the rest.
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf(
					"token: unexpected signing method: %s",
					t.Header["alg"],
				)
			}

			return []byte(secret), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("token: invalid access token: %w", err)
	}

	if !parsed.Valid {
		return nil, errors.New("token: invalid access token")
	}

	if claims.Subject == "" {
		return nil, errors.New("token: subject is missing")
	}

	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, fmt.Errorf("token: invalid subject: %w", err)
	}

	// sid is optional for backward compatibility, but when present it must
	// be a valid session UUID.
	if claims.SessionID != "" {
		if _, err := uuid.Parse(claims.SessionID); err != nil {
			return nil, fmt.Errorf("token: invalid session id: %w", err)
		}
	}

	return claims, nil
}
