package shortcode

import (
	"crypto/rand"
	"errors"
	"math/big"
)

// Base62 character set: URL-safe, alphanumeric, collision-resistant.
const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
var charsetLen = big.NewInt(int64(len(charset)))

// Generate generates a cryptographically random short code of specified length.
func Generate(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be positive")
	}

	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}

	return string(result), nil
}

// GenerateDefault generates a 6-character random short code.
func GenerateDefault() (string, error) {
	return Generate(6)
}
