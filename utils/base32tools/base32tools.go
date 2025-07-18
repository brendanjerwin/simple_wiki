package base32tools

import "encoding/base32"

func EncodeToBase32(s string) string {
	return EncodeBytesToBase32([]byte(s))
}

func EncodeBytesToBase32(s []byte) string {
	return base32.StdEncoding.EncodeToString(s)
}

func DecodeFromBase32(s string) (s2 string, err error) {
	bString, err := base32.StdEncoding.DecodeString(s)
	s2 = string(bString)
	return s2, err
}
