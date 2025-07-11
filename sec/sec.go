package sec

import (
	"encoding/hex"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword generates a bcrypt hash of the password using work factor 14.
// https://github.com/gtank/cryptopasta/blob/master/hash.go
func HashPassword(password string) (string, error) {
	cost := 14
	if os.Getenv("TEST_ENV") == "true" {
		cost = bcrypt.MinCost
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash), nil
}

// CheckPassword securely compares a bcrypt hashed password with its possible
// plaintext equivalent.  Returns nil on success, or an error on failure.
// https://github.com/gtank/cryptopasta/blob/master/hash.go
func CheckPasswordHash(password, hashedString string) error {
	hash, err := hex.DecodeString(hashedString)
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(password))
}
