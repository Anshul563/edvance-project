package token

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// accessTokenAlgorithm is the only JWT signing algorithm we accept.
const accessTokenAlgorithm = "HS256"

// ValidateAccessToken verifies signature, expiry, issuer, audience and
// algorithm, and returns the claims. Only HS256 is accepted. This service
// never signs tokens — validation only.
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

	return claims, nil
}
