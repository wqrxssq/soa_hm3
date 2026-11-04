package svcjwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	Issuer      = "booking"
	Audience    = "flight"
	MetadataKey = "authorization"
	TTL         = 5 * time.Minute
	Leeway      = 30 * time.Second
)

func Sign(secret string, now time.Time) (string, error) {
	if secret == "" {
		return "", errors.New("empty jwt secret")
	}
	claims := jwt.RegisteredClaims{
		Issuer:    Issuer,
		Audience:  jwt.ClaimStrings{Audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(TTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func Parse(secret, token string, now time.Time) error {
	if secret == "" || token == "" {
		return errors.New("empty jwt")
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(Audience),
		jwt.WithLeeway(Leeway),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	_, err := parser.Parse(token, func(parsed *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	return err
}
