package controllers

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/mfuadfakhruzzaki/backend-api/config"
	"github.com/mfuadfakhruzzaki/backend-api/middleware"
	"github.com/mfuadfakhruzzaki/backend-api/models"
	"gorm.io/gorm"
)

// UploadProfilePicture handles the upload of a user's profile picture
// @Summary Upload profile picture
// @Description Upload a profile picture for the currently logged-in user
// @Tags User
// @Accept multipart/form-data
// @Produce json
// @Param profile_picture formData file true "Profile picture file (jpg, jpeg, png)"
// @Success 200 {object} map[string]interface{} "Profile picture uploaded successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request or file type"
// @Failure 401 {object} map[string]interface{} "Unauthorized or email not found"
// @Failure 403 {object} map[string]interface{} "Email not verified"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /users/profile/picture [post]
func UploadProfilePicture(c *gin.Context) {
	// Retrieve the user's email from the context (set by JWT middleware)
	email, exists := c.Get(string(middleware.UserContextKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Email not found in context"})
		return
	}

	emailStr, ok := email.(string)
	if !ok || emailStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid email in context"})
		return
	}

	// Find the user in the database based on email
	var user models.User
	result := config.DB.Where("email = ?", emailStr).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	// Check if the email is verified before allowing profile picture upload
	if !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Email not verified. Please verify your email to upload a profile picture."})
		return
	}

	// Parse the multipart form with a maximum memory of 10MB
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing form data"})
		return
	}

	// Retrieve the file from the form input named "profile_picture"
	file, handler, err := c.Request.FormFile("profile_picture")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error retrieving file"})
		return
	}
	defer file.Close()

	// Validate the file extension
	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}
	fileExt := filepath.Ext(handler.Filename)
	if !allowedExtensions[fileExt] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only JPG, JPEG, and PNG are allowed."})
		return
	}

	// Nama file yang akan di-upload ke Google Cloud Storage
	objectName := fmt.Sprintf("uploads/profile_pictures/user_%d%s", user.ID, fileExt)
	bucketName := os.Getenv("GCS_BUCKET_NAME")

	// Upload file ke Google Cloud Storage
	if err := uploadToCloudStorage(bucketName, objectName, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload profile picture to cloud storage"})
		return
	}

	// Update the user's ProfilePicture field with the new file path in GCS
	user.ProfilePicture = fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectName)
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating user profile"})
		return
	}

	// Return a success response
	c.JSON(http.StatusOK, gin.H{
		"message":         "Profile picture uploaded successfully",
		"profile_picture": user.ProfilePicture,
	})
}

// uploadToCloudStorage is a helper function to upload files to Google Cloud Storage
func uploadToCloudStorage(bucketName, objectName string, file multipart.File) error {
	ctx := context.Background()

	// Create a Google Cloud Storage client
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	// Upload the file to the bucket
	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	if _, err := io.Copy(wc, file); err != nil {
		return fmt.Errorf("failed to upload file to cloud storage: %v", err)
	}

	// Close the writer to complete the upload
	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close storage writer: %v", err)
	}

	return nil
}

// GetProfile returns the profile data of the currently logged-in user
func GetProfile(c *gin.Context) {
	email, exists := c.Get(string(middleware.UserContextKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Email not found in context"})
		return
	}

	emailStr, ok := email.(string)
	if !ok || emailStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid email in context"})
		return
	}

	// Find the user in the database based on email
	var user models.User
	result := config.DB.Where("email = ?", emailStr).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	// Check if the email is verified
	if !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Email not verified. Please verify your email to view profile."})
		return
	}

	// Remove password from response for security reasons
	user.Password = ""

	// Return the user's profile data as JSON
	c.JSON(http.StatusOK, user)
}

// UpdateProfile handles updating the user's profile information
func UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	email, exists := c.Get(string(middleware.UserContextKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Email not found in context"})
		return
	}

	emailStr, ok := email.(string)
	if !ok || emailStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid email in context"})
		return
	}

	var user models.User
	result := config.DB.Where("email = ?", emailStr).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	updated := false
	if req.Username != "" && req.Username != user.Username {
		user.Username = req.Username
		updated = true
	}
	if req.PhoneNumber != "" && req.PhoneNumber != user.PhoneNumber {
		user.PhoneNumber = req.PhoneNumber
		updated = true
	}
	if req.Email != "" && req.Email != user.Email {
		user.Email = req.Email
		user.EmailVerified = false
		updated = true
	}

	if !updated {
		c.JSON(http.StatusOK, gin.H{"message": "No changes detected"})
		return
	}

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"data":    user,
	})
}

// UpdateUsername handles updating the user's username
func UpdateUsername(c *gin.Context) {
	var req UpdateUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	email, exists := c.Get(string(middleware.UserContextKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Email not found in context"})
		return
	}

	emailStr, ok := email.(string)
	if !ok || emailStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid email in context"})
		return
	}

	var user models.User
	result := config.DB.Where("email = ?", emailStr).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	user.Username = req.Username
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update username"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Username updated successfully",
		"username": user.Username,
	})
}

// UpdatePhoneNumber handles updating the user's phone number
func UpdatePhoneNumber(c *gin.Context) {
	var req UpdatePhoneNumberRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PhoneNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	email, exists := c.Get(string(middleware.UserContextKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Email not found in context"})
		return
	}

	emailStr, ok := email.(string)
	if !ok || emailStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid email in context"})
		return
	}

	var user models.User
	result := config.DB.Where("email = ?", emailStr).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	user.PhoneNumber = req.PhoneNumber
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update phone number"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Phone number updated successfully",
		"phoneNumber": user.PhoneNumber,
	})
}

// UpdateProfileRequest represents the JSON structure for updating user profile
type UpdateProfileRequest struct {
	Username    string `json:"username"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
}

// UpdateUsernameRequest represents the JSON structure for updating username
type UpdateUsernameRequest struct {
	Username string `json:"username"`
}

// UpdatePhoneNumberRequest represents the JSON structure for updating phone number
type UpdatePhoneNumberRequest struct {
	PhoneNumber string `json:"phone_number"`
}
