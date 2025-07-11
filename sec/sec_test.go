package sec

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sec Suite")
}

var _ = Describe("HashPassword and CheckPasswordHash", func() {

	var (
		password       string
		hashedPassword string
		err            error
	)

	Describe("with a valid password hash", func() {

		BeforeEach(func() {
			password = "mySecurePassword"
			var hashErr error
			hashedPassword, hashErr = HashPassword(password)
			Expect(hashErr).NotTo(HaveOccurred())
		})

		When("the password is correct", func() {

			BeforeEach(func() {
				err = CheckPasswordHash(password, hashedPassword)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the password is incorrect", func() {

			BeforeEach(func() {
				err = CheckPasswordHash("wrongPassword", hashedPassword)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("crypto/bcrypt: hashedPassword is not the hash of the given password"))
			})
		})
	})

	When("the password is blank", func() {

		BeforeEach(func() {
			password = ""
			var hashErr error
			hashedPassword, hashErr = HashPassword(password)
			Expect(hashErr).NotTo(HaveOccurred())

			err = CheckPasswordHash(password, hashedPassword)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the hashed password is blank", func() {

		BeforeEach(func() {
			password = "mySecurePassword"

			err = CheckPasswordHash(password, "")
		})

		It("should return an error", func() {
			Expect(err).To(MatchError("crypto/bcrypt: hashedSecret too short to be a bcrypted password"))
		})
	})

	When("the password is too long", func() {

		BeforeEach(func() {
			// A password longer than 72 bytes, which is bcrypt's limit.
			password = "This password is way too long to be hashed by bcrypt, it should be more than 72 bytes"
			Expect(len(password)).To(BeNumerically(">", 72))
			var hashErr error
			hashedPassword, hashErr = HashPassword(password)
			if hashErr != nil {
				err = hashErr
			} else {
				err = CheckPasswordHash(password, hashedPassword)
			}
		})

		It("should return an error because bcrypt cannot handle passwords longer than 72 bytes", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bcrypt"))
		})
	})

	When("the hashed string is not a valid hex string", func() {

		BeforeEach(func() {
			err = CheckPasswordHash("any password", "not-a-valid-hex-string")
		})

		It("should return an error", func() {
			Expect(err).To(MatchError("encoding/hex: invalid byte: U+006E 'n'"))
		})
	})
})
