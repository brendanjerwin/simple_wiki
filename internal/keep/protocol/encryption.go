package protocol

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // SHA-1 is part of the Google wire protocol; not used for security.
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
)

// b64AndroidKey is the base64-encoded Android Play Services 7.3.29 RSA
// public key. Format: 4 BE bytes modulus length, modulus, 4 BE bytes
// exponent length, exponent. See REFERENCE.md "Encrypted password
// construction".
const b64AndroidKey = "AAAAgMom/1a/v0lblO2Ubrt60J2gcuXSljGFQXgcyZWveWLEwo6prwgi3iJIZdodyhKZQrNWp5nKJ3srRXcUW+F1BD3baEVGcmEgqaLZUNBjm057pKRI16kB0YppeGx5qIQ5QjKzsR8ETQbKLNWgRY0QRNVz34kMJR3P/LgHax/6rmf5AAAAAwEAAQ=="

const (
	// nullByte separates email and secret in the RSA-OAEP plaintext
	// (matches the gpsoauth Python implementation byte-for-byte).
	nullByte = 0x00

	// versionByte is the leading byte of the EncryptedPasswd packet.
	versionByte = 0x00

	// fingerprintLen is how many leading SHA-1 bytes of the key struct
	// Google uses as the key fingerprint in the wire format.
	fingerprintLen = 4

	// lenPrefixBytes is the size of each big-endian length prefix in the
	// gpsoauth key serialization.
	lenPrefixBytes = 4
)

// androidKey is parsed once at package init.
var androidKey *rsa.PublicKey

func init() {
	k, err := parseAndroidKey(b64AndroidKey)
	if err != nil {
		panic(fmt.Sprintf("keep/protocol: invalid embedded Android key: %v", err))
	}
	androidKey = k
}

// encryptCredential builds the EncryptedPasswd field: 1 version byte +
// 4-byte SHA-1 fingerprint of the key struct + RSA-OAEP-SHA1 ciphertext of
// (email + 0x00 + secret), then urlsafe-base64. See REFERENCE.md.
func encryptCredential(key *rsa.PublicKey, email, secret string) (string, error) {
	plain := make([]byte, 0, len(email)+1+len(secret))
	plain = append(plain, []byte(email)...)
	plain = append(plain, nullByte)
	plain = append(plain, []byte(secret)...)

	cipher, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, key, plain, nil) //nolint:gosec // SHA-1 is mandated by the Google wire protocol.
	if err != nil {
		return "", err
	}

	fp := keyFingerprint(key)

	out := make([]byte, 0, 1+fingerprintLen+len(cipher))
	out = append(out, versionByte)
	out = append(out, fp[:fingerprintLen]...)
	out = append(out, cipher...)

	return base64.URLEncoding.EncodeToString(out), nil
}

// keyFingerprint returns SHA-1 of the packed key struct used by the gpsoauth
// reference implementation.
func keyFingerprint(key *rsa.PublicKey) [sha1.Size]byte {
	mod := key.N.Bytes()
	exp := big.NewInt(int64(key.E)).Bytes()

	buf := make([]byte, 0, lenPrefixBytes+len(mod)+lenPrefixBytes+len(exp))
	buf = append(buf, beUint32(uint32(len(mod)))...)
	buf = append(buf, mod...)
	buf = append(buf, beUint32(uint32(len(exp)))...)
	buf = append(buf, exp...)

	return sha1.Sum(buf) //nolint:gosec // Fingerprint is part of the Google wire protocol.
}

// parseAndroidKey decodes the gpsoauth-format base64 RSA public key into a
// usable *rsa.PublicKey. The serialization is gpsoauth's, not PEM:
// 4 BE bytes modLen, modLen bytes modulus, 4 BE bytes expLen, expLen bytes
// exponent.
func parseAndroidKey(b64 string) (*rsa.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(raw) < lenPrefixBytes {
		return nil, errors.New("key too short for modulus length prefix")
	}
	modLen := int(parseBeUint32(raw[0:lenPrefixBytes]))
	if lenPrefixBytes+modLen > len(raw) {
		return nil, errors.New("key too short for modulus")
	}
	mod := new(big.Int).SetBytes(raw[lenPrefixBytes : lenPrefixBytes+modLen])

	rest := raw[lenPrefixBytes+modLen:]
	if len(rest) < lenPrefixBytes {
		return nil, errors.New("key too short for exponent length prefix")
	}
	expLen := int(parseBeUint32(rest[0:lenPrefixBytes]))
	if lenPrefixBytes+expLen > len(rest) {
		return nil, errors.New("key too short for exponent")
	}
	exp := new(big.Int).SetBytes(rest[lenPrefixBytes : lenPrefixBytes+expLen])
	if !exp.IsInt64() {
		return nil, errors.New("exponent does not fit in int64")
	}
	return &rsa.PublicKey{N: mod, E: int(exp.Int64())}, nil
}

// beUint32 encodes v as 4 big-endian bytes.
func beUint32(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// parseBeUint32 reads 4 big-endian bytes as a uint32. Caller must guarantee
// at least 4 bytes are present.
func parseBeUint32(b []byte) uint32 {
	const (
		byte0Shift = 24
		byte1Shift = 16
		byte2Shift = 8
	)
	return uint32(b[0])<<byte0Shift | uint32(b[1])<<byte1Shift | uint32(b[2])<<byte2Shift | uint32(b[3])
}
