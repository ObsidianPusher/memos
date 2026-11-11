package v1

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

func parseAccessTokenClaims(accessToken, secret string) (*ClaimsMessage, error) {
	claims := &ClaimsMessage{}
	_, err := jwt.ParseWithClaims(accessToken, claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Name {
			return nil, fmt.Errorf("unexpected access token signing method=%v, expect %v", t.Header["alg"], jwt.SigningMethodHS256)
		}
		if kid, ok := t.Header["kid"].(string); ok && kid == "v1" {
			return []byte(secret), nil
		}
		return nil, fmt.Errorf("unexpected access token kid=%v", t.Header["kid"])
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse access token: %w", err)
	}
	return claims, nil
}
