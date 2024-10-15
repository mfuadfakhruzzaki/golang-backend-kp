// controllers/userController.go
package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/mfuadfakhruzzaki/backend-api/config"
	"github.com/mfuadfakhruzzaki/backend-api/middleware"
	"github.com/mfuadfakhruzzaki/backend-api/models"
	"gorm.io/gorm"
)

// UploadProfilePicture mengelola unggahan gambar profil pengguna
func UploadProfilePicture(c *gin.Context) {
	// Mengambil email pengguna dari context (ditetapkan oleh middleware JWT)
	email, exists := c.Get(string(middleware.UserContextKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Email tidak ditemukan dalam context"})
		return
	}

	emailStr, ok := email.(string)
	if !ok || emailStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Email tidak valid dalam context"})
		return
	}

	// Mencari pengguna di database berdasarkan email
	var user models.User
	result := config.DB.Where("email = ?", emailStr).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	// Memeriksa apakah email telah diverifikasi sebelum mengizinkan unggahan gambar profil
	if !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Email belum diverifikasi. Silakan verifikasi email Anda untuk mengunggah gambar profil."})
		return
	}

	// Mengambil file dari form data dengan key "profile_picture"
	fileHeader, err := c.FormFile("profile_picture")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Error retrieving file: %v", err)})
		return
	}

	// Memeriksa ekstensi file yang diunggah
	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}
	fileExt := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedExtensions[fileExt] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Hanya JPG, JPEG, dan PNG yang diperbolehkan."})
		return
	}

	// Membuka file yang diunggah
	uploadedFile, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error opening file: %v", err)})
		return
	}
	defer uploadedFile.Close()

	// Mendefinisikan nama objek dan bucket GCS
	objectName := fmt.Sprintf("uploads/profile_pictures/user_%d%s", user.ID, fileExt)
	bucketName := os.Getenv("GCS_BUCKET_NAME")
	if bucketName == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server configuration error: GCS_BUCKET_NAME tidak diatur"})
		return
	}

	// (Opsional) Menghapus gambar profil sebelumnya jika ada
	if user.ProfilePicture != "" {
		parts := strings.Split(user.ProfilePicture, "/")
		if len(parts) >= 5 {
			objectNameOld := strings.Join(parts[4:], "/") // Sesuaikan berdasarkan struktur URL Anda
			err := deleteFromCloudStorage(bucketName, objectNameOld)
			if err != nil {
				// Mencatat error tetapi tidak mencegah unggahan
				fmt.Printf("Gagal menghapus gambar profil lama: %v\n", err)
			}
		}
	}

	// Mengunggah file ke Google Cloud Storage
	if err := uploadToCloudStorage(bucketName, objectName, uploadedFile, fileExt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Gagal mengunggah gambar profil ke cloud storage: %v", err)})
		return
	}

	// Memperbarui field ProfilePicture pengguna dengan URL baru di GCS
	user.ProfilePicture = fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectName)
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating user profile"})
		return
	}

	// Mengembalikan respons sukses
	c.JSON(http.StatusOK, gin.H{
		"message":         "Profile picture uploaded successfully",
		"profile_picture": user.ProfilePicture,
	})
}

// uploadToCloudStorage mengunggah file ke Google Cloud Storage
func uploadToCloudStorage(bucketName, objectName string, reader io.Reader, fileExt string) error {
	ctx := context.Background()

	// Membuat klien Google Cloud Storage
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("gagal membuat storage client: %v", err)
	}
	defer client.Close()

	// Membuat writer untuk bucket dan objek yang ditentukan
	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)

	// Menetapkan ContentType yang sesuai berdasarkan ekstensi file
	contentType := ""
	switch fileExt {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	default:
		contentType = "application/octet-stream"
	}
	wc.ContentType = contentType

	// Menyalin data file ke writer
	if _, err := io.Copy(wc, reader); err != nil {
		return fmt.Errorf("gagal mengunggah file ke cloud storage: %v", err)
	}

	// Menutup writer untuk menyelesaikan unggahan
	if err := wc.Close(); err != nil {
		return fmt.Errorf("gagal menutup storage writer: %v", err)
	}

	return nil
}

// deleteFromCloudStorage menghapus file dari Google Cloud Storage
func deleteFromCloudStorage(bucketName, objectName string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("gagal membuat storage client: %v", err)
	}
	defer client.Close()

	obj := client.Bucket(bucketName).Object(objectName)
	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("gagal menghapus objek dari cloud storage: %v", err)
	}

	return nil
}

// GetProfile adalah contoh fungsi untuk mendapatkan profil pengguna
func GetProfile(c *gin.Context) {
	// Implementasikan sesuai kebutuhan Anda
	c.JSON(http.StatusOK, gin.H{"message": "GetProfile not implemented"})
}

// UpdateProfile adalah contoh fungsi untuk memperbarui profil pengguna
func UpdateProfile(c *gin.Context) {
	// Implementasikan sesuai kebutuhan Anda
	c.JSON(http.StatusOK, gin.H{"message": "UpdateProfile not implemented"})
}

// UpdateUsername adalah contoh fungsi untuk memperbarui username pengguna
func UpdateUsername(c *gin.Context) {
	// Implementasikan sesuai kebutuhan Anda
	c.JSON(http.StatusOK, gin.H{"message": "UpdateUsername not implemented"})
}

// UpdatePhoneNumber adalah contoh fungsi untuk memperbarui nomor telepon pengguna
func UpdatePhoneNumber(c *gin.Context) {
	// Implementasikan sesuai kebutuhan Anda
	c.JSON(http.StatusOK, gin.H{"message": "UpdatePhoneNumber not implemented"})
}
