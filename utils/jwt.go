package utils

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func GenerateJWT(userID uuid.UUID) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET is empty")
	}
	expHours := 24
	if raw := os.Getenv("JWT_EXPIRE_HOURS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			expHours = parsed
		}
	}

	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(time.Duration(expHours) * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateJWT(tokenString string) (uuid.UUID, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return uuid.Nil, fmt.Errorf("JWT_SECRET is empty")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token")
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("user_id missing in token")
	}
	return uuid.Parse(userIDStr)
}
