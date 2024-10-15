// middleware/authMiddleware.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mfuadfakhruzzaki/backend-api/utils"
)

type ContextKey string

const (
	UserContextKey ContextKey = "userEmail"
	AuthHeader     string     = "Authorization"
	BearerSchema   string     = "bearer"
)

// JWTMiddleware memverifikasi token JWT dan menambahkan email pengguna ke context Gin
func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Mengambil header Authorization
		authHeader := c.GetHeader(AuthHeader)
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			c.Abort()
			return
		}

		// Memisahkan bagian "Bearer" dan token
		// Format yang diharapkan: "Bearer <token>"
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || strings.ToLower(tokenParts[0]) != BearerSchema {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format. Expected 'Bearer <token>'"})
			c.Abort()
			return
		}

		// Mengambil token
		tokenString := tokenParts[1]

		// Memvalidasi token dan mengambil email
		email, err := utils.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token: " + err.Error()})
			c.Abort()
			return
		}

		// Menyimpan email ke context
		c.Set(string(UserContextKey), email)

		// Melanjutkan ke handler berikutnya
		c.Next()
	}
}
