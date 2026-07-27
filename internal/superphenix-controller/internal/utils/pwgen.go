package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	letterBytes  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	specialBytes = "!@#$%^&*()_+-=[]{}|;:.<>/?~"
	numBytes     = "0123456789"
)

func GeneratePassword(length int) (string, error) {
	b := make([]byte, length)

	charset := letterBytes + numBytes + specialBytes
	charsetLen := big.NewInt(int64(len(charset)))

	for i := range b {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate char")
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}
