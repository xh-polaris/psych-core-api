package random

import (
	"crypto/rand"
	"math/big"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
const accountLength = 8 // 账号长度

// GenerateRandomAccount 随机生成一个账号
func GenerateRandomAccount() (string, error) {
	result := make([]byte, accountLength)
	for i := range result {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[index.Int64()]
	}
	return string(result), nil
}
