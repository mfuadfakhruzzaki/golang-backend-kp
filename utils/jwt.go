// utils/jwt.go
package utils

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
)

var (
	ErrSecretNotSet = errors.New("JWT_SECRET is not set in the environment")
)

// Claims defines the structure for JWT claims
type Claims struct {
	Email string `json:"email"`
	jwt.StandardClaims
}

// GenerateJWT membuat token JWT berdasarkan email pengguna
func GenerateJWT(email string) (string, error) {
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		return "", ErrSecretNotSet
	}
	mySigningKey := []byte(secretKey)

	// Membuat klaim JWT, termasuk email dan waktu kadaluarsa
	claims := &Claims{
		Email: email,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    "your-app-name", // Ganti dengan nama aplikasi Anda
		},
	}

	// Membuat token dengan klaim yang telah ditetapkan
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Menandatangani token
	tokenString, err := token.SignedString(mySigningKey)
	if err != nil {
		return "", err
	}

	fmt.Println("Token generated:", tokenString) // Output debugging
	return tokenString, nil
}

// ValidateToken memvalidasi token JWT dan mengembalikan email pengguna jika valid
func ValidateToken(tokenString string) (string, error) {
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		return "", ErrSecretNotSet
	}
	mySigningKey := []byte(secretKey)

	// Parse token
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Memastikan metode penandatanganan sesuai
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return mySigningKey, nil
	})

	if err != nil {
		return "", err
	}

	if !token.Valid {
		return "", errors.New("invalid token")
	}

	// Token valid, kembalikan email
	return claims.Email, nil
}
