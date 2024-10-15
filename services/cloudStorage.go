// services/cloudStorage.go
package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"

	"cloud.google.com/go/storage"
)

// UploadToCloudStorage mengupload file ke Google Cloud Storage
func UploadToCloudStorage(bucketName, objectName string, file multipart.File) error {
	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Failed to create storage client: %v", err)
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	// Menyimpan file ke bucket
	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	wc.ContentType = "application/octet-stream"
	wc.CacheControl = "public, max-age=86400" // Contoh header tambahan

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

// GetFileFromCloudStorage mengambil file dari Google Cloud Storage
func GetFileFromCloudStorage(bucketName, objectName string) ([]byte, error) {
	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	rc, err := client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("Object.NewReader: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("io.ReadAll: %v", err)
	}

	return data, nil
}
