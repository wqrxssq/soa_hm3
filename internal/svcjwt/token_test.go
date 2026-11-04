package svcjwt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignAndParse(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	secret := "secret"
	token, err := Sign(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := Parse(secret, token, now); err != nil {
		t.Fatal(err)
	}
	if err := Parse("other", token, now); err == nil {
		t.Fatal("чужой секрет принят")
	}
	if _, err := Sign("", now); err == nil {
		t.Fatal("пустой секрет подписан")
	}
}

func TestParseLeeway(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	secret := "secret"
	within, err := Sign(secret, now.Add(-TTL-20*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := Parse(secret, within, now); err != nil {
		t.Fatal(err)
	}
	beyond, err := Sign(secret, now.Add(-TTL-45*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := Parse(secret, beyond, now); err == nil {
		t.Fatal("просроченный токен принят")
	}
}

func TestParseRejectsClaims(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	secret := "secret"
	tests := []struct {
		name   string
		claims jwt.RegisteredClaims
		method jwt.SigningMethod
	}{
		{
			name:   "issuer",
			method: jwt.SigningMethodHS256,
			claims: jwt.RegisteredClaims{
				Issuer:    "other",
				Audience:  jwt.ClaimStrings{Audience},
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(TTL)),
			},
		},
		{
			name:   "audience",
			method: jwt.SigningMethodHS256,
			claims: jwt.RegisteredClaims{
				Issuer:    Issuer,
				Audience:  jwt.ClaimStrings{"other"},
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(TTL)),
			},
		},
		{
			name:   "no expiry",
			method: jwt.SigningMethodHS256,
			claims: jwt.RegisteredClaims{
				Issuer:   Issuer,
				Audience: jwt.ClaimStrings{Audience},
				IssuedAt: jwt.NewNumericDate(now),
			},
		},
		{
			name:   "hs384",
			method: jwt.SigningMethodHS384,
			claims: jwt.RegisteredClaims{
				Issuer:    Issuer,
				Audience:  jwt.ClaimStrings{Audience},
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(TTL)),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := jwt.NewWithClaims(test.method, test.claims)
			signed, err := token.SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if err := Parse(secret, signed, now); err == nil {
				t.Fatal("токен принят")
			}
		})
	}
}

func TestParseRejectsNoneAlgorithm(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Issuer:    Issuer,
		Audience:  jwt.ClaimStrings{Audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(TTL)),
	})
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if err := Parse("secret", signed, now); err == nil {
		t.Fatal("alg=none принят")
	}
}
