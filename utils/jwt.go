package utils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt"
)

func VerifyJWT(tokenString, secret string) (map[string]any, error) {
	tokenString = strings.TrimSpace(tokenString)

	parser := jwt.Parser{SkipClaimsValidation: true}
	token, err := parser.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("JWT parsing failed: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
