// Package sec provides security related utilities.
package sec

import (
	"encoding/hex"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword generates a bcrypt hash of the password using work factor 14.
// https://github.com/gtank/cryptopasta/blob/master/hash.go
const bcryptCost = 14

func HashPassword(password string) string {
	cost := bcryptCost
	if os.Getenv("TEST_ENV") == "true" {
		cost = bcrypt.MinCost
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), cost)
	return hex.EncodeToString(hash)
}

// CheckPasswordHash securely compares a bcrypt hashed password with its possible
// plaintext equivalent.  Returns nil on success, or an error on failure.
// https://github.com/gtank/cryptopasta/blob/master/hash.go
func CheckPasswordHash(password, hashedString string) error {
	hash, err := hex.DecodeString(hashedString)
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(password))
}
