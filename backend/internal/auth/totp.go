package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type TOTPService struct{}

func NewTOTPService() *TOTPService {
	return &TOTPService{}
}

func (s *TOTPService) GenerateSecret(issuer, email string) (*otp.Key, error) {
	return totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: email,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
}

func (s *TOTPService) ValidateCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateRecoveryCodes returns (plaintext codes, hashed codes).
func (s *TOTPService) GenerateRecoveryCodes(count int) ([]string, []string, error) {
	plain := make([]string, count)
	hashed := make([]string, count)
	for i := 0; i < count; i++ {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, fmt.Errorf("failed to generate recovery code: %w", err)
		}
		code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
		plain[i] = code
		hash := sha256.Sum256([]byte(code))
		hashed[i] = base64.StdEncoding.EncodeToString(hash[:])
	}
	return plain, hashed, nil
}

func (s *TOTPService) ValidateRecoveryCode(code string, hashedCodes []string) (int, bool) {
	hash := sha256.Sum256([]byte(code))
	codeHash := base64.StdEncoding.EncodeToString(hash[:])
	for i, h := range hashedCodes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(codeHash)) == 1 {
			return i, true
		}
	}
	return -1, false
}

// ValidateCodeWithWindow validates TOTP with a small time window for clock skew.
func (s *TOTPService) ValidateCodeWithWindow(secret, code string) bool {
	valid, _ := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:     1,
		Digits:   otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return valid
}
