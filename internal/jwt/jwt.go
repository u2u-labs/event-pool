package jwt

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JwtClaims struct {
	Address string `json:"address"`
	jwt.RegisteredClaims
}

func ValidateJWT(tokenStr string, secret []byte, handleValidToken func(token string) error) (*JwtClaims, error) {
	if strings.HasPrefix(tokenStr, "Bearer ") {
		tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &JwtClaims{}, func(token *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(*JwtClaims); ok && token.Valid {
		if handleValidToken != nil {
			if err := handleValidToken(tokenStr); err != nil {
				return nil, err
			}
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

func GenerateJWT(address string, expiry int64, secret []byte) (string, error) {
	claims := JwtClaims{
		Address: address,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiry) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}
