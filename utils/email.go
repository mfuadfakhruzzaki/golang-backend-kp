// utils/email.go
package utils

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// GenerateVerificationCode menghasilkan kode verifikasi 6 karakter alfanumerik menggunakan crypto/rand
func GenerateVerificationCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b), nil
}

// SendVerificationEmail mengirimkan email verifikasi dengan kode ke pengguna menggunakan gomail.v2
func SendVerificationEmail(recipientEmail string, verificationCode string) error {
	// Mengambil konfigurasi SMTP dari environment variables
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	senderEmail := os.Getenv("SMTP_SENDER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	// Memeriksa apakah semua konfigurasi SMTP telah diatur
	if smtpHost == "" || smtpPort == "" || senderEmail == "" || smtpPassword == "" {
		return fmt.Errorf("SMTP configuration is missing in environment variables")
	}

	// Membuat pesan email
	m := gomail.NewMessage()
	m.SetHeader("From", senderEmail)
	m.SetHeader("To", recipientEmail)
	m.SetHeader("Subject", "Email Verification for Data Quota Tracker")
	m.SetBody("text/plain", fmt.Sprintf(
		"Welcome to Data Quota Tracker!\n\nYour verification code is: %s\n\nPlease enter this code to verify your email and start using the app.",
		verificationCode))

	// Mengonversi SMTP_PORT dari string ke integer
	port, err := strconv.Atoi(smtpPort)
	if err != nil {
		log.Printf("Invalid SMTP port: %v", err)
		return fmt.Errorf("invalid SMTP port: %v", err)
	}

	// Membuat dialer SMTP
	d := gomail.NewDialer(smtpHost, port, senderEmail, smtpPassword)

	// Jika menggunakan port 465, aktifkan SSL
	if port == 465 {
		d.SSL = true
	}

	// Kirim email
	if err := d.DialAndSend(m); err != nil {
		log.Printf("Failed to send verification email to %s: %v", recipientEmail, err)
		return err
	}

	log.Printf("Verification email sent to %s with code %s\n", recipientEmail, verificationCode)
	return nil
}
