package sec

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	password := "mySecurePassword"

	// Hash the password
	hashedPassword := HashPassword(password)

	// Check the hashed password
	err := CheckPasswordHash(password, hashedPassword)

	if err != nil {
		t.Errorf("Failed to verify the password: %v", err)
	}
}

func TestHashAndCheckPasswordEdgeCases(t *testing.T) {
	// Test with incorrect password
	password := "mySecurePassword"
	wrongPassword := "wrongPassword"
	hashedPassword := HashPassword(password)
	err := CheckPasswordHash(wrongPassword, hashedPassword)
	if err == nil {
		t.Errorf("Expected an error when checking the wrong password, but didn't get one")
	}

	// Test with blank password
	blankPassword := ""
	hashedBlankPassword := HashPassword(blankPassword)
	err = CheckPasswordHash(blankPassword, hashedBlankPassword)
	if err != nil {
		t.Errorf("Failed to verify the blank password: %v", err)
	}

	// Test with blank hashed password
	err = CheckPasswordHash(password, "")
	if err == nil {
		t.Errorf("Expected an error when checking with a blank hashed password, but didn't get one")
	}
}
