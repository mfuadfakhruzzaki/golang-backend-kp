package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv" // Untuk memuat file .env
	"github.com/mfuadfakhruzzaki/backend-api/config"
	"github.com/mfuadfakhruzzaki/backend-api/routes"
	"github.com/mfuadfakhruzzaki/backend-api/seeds"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// uploadToCloudStorage mengupload file ke Google Cloud Storage
func uploadToCloudStorage(bucketName, objectName string, file multipart.File) error {
	ctx := context.Background()

	// Membuat client Google Cloud Storage
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Failed to create storage client: %v", err)
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	// Menyimpan file ke bucket
	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	if _, err = io.Copy(wc, file); err != nil {
		log.Printf("Failed to copy file to GCS: %v", err)
		return fmt.Errorf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		log.Printf("Failed to close writer: %v", err)
		return fmt.Errorf("Writer.Close: %v", err)
	}

	log.Printf("File successfully uploaded to %s/%s", bucketName, objectName)
	return nil
}

// getFileFromCloudStorage mengambil file dari Google Cloud Storage
func getFileFromCloudStorage(bucketName, objectName string) ([]byte, error) {
	ctx := context.Background()

	// Membuat client Google Cloud Storage
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	// Mendapatkan objek dari bucket
	rc, err := client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("Object.NewReader: %v", err)
	}
	defer rc.Close()

	// Membaca konten objek
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("io.ReadAll: %v", err)
	}

	return data, nil
}

func main() {
	// Memuat variabel environment dari .env
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Memastikan GOOGLE_APPLICATION_CREDENTIALS diatur
	googleCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if googleCreds == "" {
		log.Fatal("GOOGLE_APPLICATION_CREDENTIALS is not set in .env")
	}

	// Memastikan GCS_BUCKET_NAME diatur
	bucketName := os.Getenv("GCS_BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("GCS_BUCKET_NAME is not set in .env")
	}

	// Menghubungkan ke database dan menjalankan migrasi di config.ConnectDatabase()
	config.ConnectDatabase()

	// Menjalankan seeding data paket
	seeds.SeedPackages()

	// Membuat router baru dengan Gin
	router := gin.Default()

	// Mendaftarkan semua route API
	routes.RegisterRoutes(router)

	// Menambahkan log untuk semua route yang terdaftar
	logRoutes(router)

	// Route untuk mengupload file ke Google Cloud Storage
	router.POST("/upload", func(c *gin.Context) {
		log.Println("Received request to upload file")

		// Ambil file dari form-data
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			log.Printf("Error retrieving file: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file is received"})
			return
		}
		defer file.Close()

		// Nama file yang akan diupload ke Cloud Storage
		objectName := "uploads/" + header.Filename
		log.Printf("Uploading file to %s/%s", bucketName, objectName)

		// Upload file ke Cloud Storage
		if err := uploadToCloudStorage(bucketName, objectName, file); err != nil {
			log.Printf("Error uploading to Cloud Storage: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Berikan respon sukses
		log.Printf("File %s uploaded successfully to %s/%s", header.Filename, bucketName, objectName)
		c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully!", "file": objectName})
	})

	// Route untuk mendapatkan file dari Cloud Storage
	router.GET("/files/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		objectName := "uploads/" + filename

		log.Printf("Retrieving file from %s/%s", bucketName, objectName)

		fileData, err := getFileFromCloudStorage(bucketName, objectName)
		if err != nil {
			log.Printf("Error retrieving file from Cloud Storage: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Mengirimkan file sebagai response dengan tipe "application/octet-stream"
		c.Data(http.StatusOK, "application/octet-stream", fileData)
	})

	// Menambahkan rute untuk Swagger UI
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Menjalankan server pada port 8080
	fmt.Println("Server berjalan pada port 8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

// logRoutes mencetak semua route yang terdaftar
func logRoutes(router *gin.Engine) {
	for _, route := range router.Routes() {
		fmt.Printf("Route registered: %s %s\n", route.Method, route.Path)
	}
}
