// main.go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv" // Untuk memuat file .env
	"github.com/mfuadfakhruzzaki/backend-api/config"
	"github.com/mfuadfakhruzzaki/backend-api/routes"
	"github.com/mfuadfakhruzzaki/backend-api/seeds"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Backend API
// @version         1.0
// @description     API untuk Mengelola Profil Pengguna

func main() {
	// Memuat variabel environment dari .env
	err := godotenv.Load()
	if err != nil {
		log.Println("Tidak menemukan file .env. Menggunakan variabel environment.")
	}

	// Memastikan GOOGLE_APPLICATION_CREDENTIALS diatur
	googleCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if googleCreds == "" {
		log.Fatal("GOOGLE_APPLICATION_CREDENTIALS tidak diatur di .env")
	}

	// Memastikan GCS_BUCKET_NAME diatur
	bucketName := os.Getenv("GCS_BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("GCS_BUCKET_NAME tidak diatur di .env")
	}

	// Menghubungkan ke database dan menjalankan migrasi di config.ConnectDatabase()
	config.ConnectDatabase()

	// Menjalankan seeding data paket
	seeds.SeedPackages()

	// Membuat router baru dengan Gin
	router := gin.Default()

	// Mengatur batas ukuran multipart form (misalnya 10 MB)
	router.MaxMultipartMemory = 10 << 20 // 10 MB

	// Mendaftarkan semua route API
	routes.RegisterRoutes(router)

	// Menambahkan log untuk semua route yang terdaftar
	logRoutes(router)

	// Menambahkan rute untuk Swagger UI
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Menjalankan server pada port 8080
	fmt.Println("Server berjalan pada port 8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Gagal menjalankan server: %v", err)
	}
}

// logRoutes mencetak semua route yang terdaftar
func logRoutes(router *gin.Engine) {
	for _, route := range router.Routes() {
		fmt.Printf("Route terdaftar: %s %s\n", route.Method, route.Path)
	}
}
