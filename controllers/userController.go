// controllers/userController.go
package controllers

import (
	"context"
	"encoding/json"
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
		result := config.DB.Where("email = ? AND deleted_at IS NULL", emailStr).First(&user)
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

	// GetProfile mengambil dan mengembalikan profil pengguna yang sedang login
	func GetProfile(c *gin.Context) {
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

		// Mencari pengguna di database berdasarkan email, preload relasi Package
		var user models.User
		result := config.DB.Preload("Package").Where("email = ? AND deleted_at IS NULL", emailStr).First(&user)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// Mengonversi field Details dari Package menjadi map[string]interface{}
		var packageDetails map[string]interface{}
		if err := json.Unmarshal(user.Package.Details, &packageDetails); err != nil {
			packageDetails = nil // Atau set ke default lain jika diperlukan
		}

		// Menyiapkan data profil yang akan dikembalikan
		profile := gin.H{
			"id":              user.ID,
			"email":           user.Email,
			"username":        user.Username,
			"phone_number":    user.PhoneNumber,
			"profile_picture": user.ProfilePicture,
			"package_id":      user.PackageID,
			"package": gin.H{
				"id":         user.Package.ID,
				"name":       user.Package.Name,
				"data":       user.Package.Data,
				"duration":   user.Package.Duration,
				"price":      user.Package.Price,
				"details":    packageDetails,
				"categories": user.Package.Categories,
				"created_at": user.Package.CreatedAt,
				"updated_at": user.Package.UpdatedAt,
			},
			"email_verified": user.EmailVerified,
			"created_at":     user.CreatedAt,
			"updated_at":     user.UpdatedAt,
		}

		// Mengembalikan respons sukses dengan data profil
		c.JSON(http.StatusOK, gin.H{
			"message": "Profile fetched successfully",
			"profile": profile,
		})
	}

	// UpdateProfile mengupdate profil pengguna secara keseluruhan
	func UpdateProfile(c *gin.Context) {
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
		result := config.DB.Where("email = ? AND deleted_at IS NULL", emailStr).First(&user)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// Mendefinisikan struktur input untuk pembaruan
		type UpdateProfileInput struct {
			Email       *string `json:"email"`
			Username    *string `json:"username"`
			PhoneNumber *string `json:"phone_number"`
			PackageID   *uint   `json:"package_id"`
		}

		var input UpdateProfileInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid input: %v", err)})
			return
		}

		// Menyiapkan map untuk pembaruan
		updates := make(map[string]interface{})

		// Memproses setiap field jika disediakan
		if input.Email != nil {
			updates["email"] = *input.Email
		}

		if input.Username != nil {
			updates["username"] = *input.Username
		}

		if input.PhoneNumber != nil {
			updates["phone_number"] = *input.PhoneNumber
		}

		if input.PackageID != nil {
			// Cek apakah PackageID valid (ada di database)
			var pkg models.Package
			if err := config.DB.Where("id = ?", *input.PackageID).First(&pkg).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Package ID tidak valid"})
				return
			}
			updates["package_id"] = *input.PackageID
		}

		// Validasi jika email diubah
		if input.Email != nil && *input.Email != user.Email {
			// Pastikan email belum digunakan oleh pengguna lain
			var existingUser models.User
			if err := config.DB.Where("email = ?", *input.Email).First(&existingUser).Error; err == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Email sudah digunakan oleh pengguna lain"})
				return
			}
			// Tandai email sebagai belum diverifikasi jika diubah
			updates["email_verified"] = false
			// Anda mungkin juga ingin mengirim email verifikasi baru di sini
		}

		// Validasi jika username diubah
		if input.Username != nil && *input.Username != user.Username {
			// Pastikan username belum digunakan oleh pengguna lain
			var existingUser models.User
			if err := config.DB.Where("username = ?", *input.Username).First(&existingUser).Error; err == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Username sudah digunakan oleh pengguna lain"})
				return
			}
		}

		// Memperbarui pengguna
		if len(updates) > 0 {
			if err := config.DB.Model(&user).Updates(updates).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating profile"})
				return
			}
		}

		// Mengembalikan respons sukses
		c.JSON(http.StatusOK, gin.H{
			"message": "Profile updated successfully",
		})
	}

	// UpdateUsername mengupdate username pengguna
	func UpdateUsername(c *gin.Context) {
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
		result := config.DB.Where("email = ? AND deleted_at IS NULL", emailStr).First(&user)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// Mendefinisikan struktur input untuk pembaruan username
		type UpdateUsernameInput struct {
			Username string `json:"username" binding:"required"`
		}

		var input UpdateUsernameInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid input: %v", err)})
			return
		}

		// Validasi apakah username sudah digunakan
		var existingUser models.User
		if err := config.DB.Where("username = ?", input.Username).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username sudah digunakan oleh pengguna lain"})
			return
		}

		// Memperbarui username
		if err := config.DB.Model(&user).Update("username", input.Username).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating username"})
			return
		}

		// Mengembalikan respons sukses
		c.JSON(http.StatusOK, gin.H{
			"message":  "Username updated successfully",
			"username": input.Username,
		})
	}

	// UpdatePhoneNumber mengupdate nomor telepon pengguna
	func UpdatePhoneNumber(c *gin.Context) {
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
		result := config.DB.Where("email = ? AND deleted_at IS NULL", emailStr).First(&user)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// Mendefinisikan struktur input untuk pembaruan nomor telepon
		type UpdatePhoneNumberInput struct {
			PhoneNumber string `json:"phone_number" binding:"required"`
		}

		var input UpdatePhoneNumberInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid input: %v", err)})
			return
		}

		// Optional: Validasi format nomor telepon
		// Misalnya, pastikan hanya angka dan panjang tertentu
		if len(input.PhoneNumber) < 10 || len(input.PhoneNumber) > 15 || !isNumeric(input.PhoneNumber) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid phone number format"})
			return
		}

		// Memperbarui nomor telepon
		if err := config.DB.Model(&user).Update("phone_number", input.PhoneNumber).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating phone number"})
			return
		}

		// Mengembalikan respons sukses
		c.JSON(http.StatusOK, gin.H{
			"message":      "Phone number updated successfully",
			"phone_number": input.PhoneNumber,
		})
	}

	// Helper function untuk memeriksa apakah string hanya terdiri dari angka
	func isNumeric(s string) bool {
		for _, c := range s {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
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
