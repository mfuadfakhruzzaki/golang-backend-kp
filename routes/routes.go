package routes

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/mfuadfakhruzzaki/backend-api/controllers"
	"github.com/mfuadfakhruzzaki/backend-api/middleware"
)

func RegisterRoutes(router *gin.Engine) {
	// Set up CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, 
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Public Routes
	public := router.Group("/")
	{
		// Registration and Login Endpoints
		public.POST("/auth/register", controllers.Register)      
		public.POST("/auth/login", controllers.Login)

		// Endpoint untuk verifikasi email
		public.POST("/auth/verify-email", controllers.VerifyEmail)

		
	}

	// Protected Routes with JWT Middleware
	api := router.Group("/api")
	api.Use(middleware.JWTMiddleware()) // JWT Middleware untuk proteksi endpoint
	{
		// Package Endpoints
		api.GET("/packages", controllers.GetPackages)               // Mendapatkan semua paket
		api.GET("/packages/:id", controllers.GetPackageByID)        // Mendapatkan satu paket berdasarkan ID
		api.POST("/packages/:id/select", controllers.SelectPackage) // Memilih paket berdasarkan ID

		// User Endpoints
		api.POST("/users/profile/picture", controllers.UploadProfilePicture)
		api.GET("/users/profile", controllers.GetProfile)

		// **Rute Baru untuk Mengupdate Profil Pengguna**
		api.PUT("/users/profile", controllers.UpdateProfile) // Mengupdate profil secara keseluruhan

		// **Rute Opsional untuk Mengupdate Username dan Nomor Telepon Secara Khusus**
		api.PUT("/users/profile/username", controllers.UpdateUsername)      // Mengupdate username
		api.PUT("/users/profile/phone_number", controllers.UpdatePhoneNumber) // Mengupdate nomor telepon
	}
}
