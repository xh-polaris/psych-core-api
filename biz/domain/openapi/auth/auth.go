// 本包提供开放接口访问密钥校验；第一期由配置驱动，不依赖数据表。
//
// 密钥格式：sk_live_<prefix>_<secret>
//   - 前缀：密钥随机部分的 SHA-256 前 8 位，用于日志关联和定位候选配置；
//   - 随机部分：随机字符串，明文只在生成命令输出时展示一次；
//   - 配置侧只保存基于服务器盐计算的 HMAC-SHA256 摘要，永不存储明文。

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/xh-polaris/psych-core-api/biz/conf"
)

const (
	keyPrefixTag   = "sk_live_"
	statusActive   = "active"
	prefixLen      = 8
	minSecretLen   = 20
	trimmedTokenSk = "token"
)

var (
	ErrInvalid           = errors.New("invalid api key")
	ErrRevoked           = errors.New("api key revoked")
	ErrInsufficientScope = errors.New("insufficient scope")
)

type Key struct {
	Prefix string
	Scopes []string
}

func Verify(pepper string, keys []conf.OpenApiKey, token, requiredScope string) (Key, error) {
	prefix, secret, err := parseToken(token)
	if err != nil {
		return Key{}, ErrInvalid
	}
	if pepper == "" || len(keys) == 0 {
		return Key{}, ErrInvalid
	}
	candidate, ok := findKey(keys, prefix)
	if !ok {
		return Key{}, ErrInvalid
	}
	if candidate.Status != statusActive {
		return Key{}, ErrRevoked
	}
	if !hmac.Equal([]byte(digest(secret, pepper)), []byte(strings.ToLower(candidate.Digest))) {
		return Key{}, ErrInvalid
	}
	if !hasScope(candidate.Scopes, requiredScope) {
		return Key{}, ErrInsufficientScope
	}
	return Key{Prefix: candidate.Prefix, Scopes: candidate.Scopes}, nil
}

func parseToken(token string) (prefix, secret string, err error) {
	token = strings.TrimSpace(token)
	rest, ok := strings.CutPrefix(token, keyPrefixTag)
	if !ok {
		return "", "", fmt.Errorf("%s: missing %s prefix", trimmedTokenSk, keyPrefixTag)
	}
	prefix, secret, ok = strings.Cut(rest, "_")
	if !ok || len(prefix) != prefixLen || len(secret) < minSecretLen {
		return "", "", fmt.Errorf("%s: malformed", trimmedTokenSk)
	}
	return prefix, secret, nil
}

func findKey(keys []conf.OpenApiKey, prefix string) (conf.OpenApiKey, bool) {
	for _, k := range keys {
		if strings.EqualFold(k.Prefix, prefix) {
			return k, true
		}
	}
	return conf.OpenApiKey{}, false
}

func digest(secret, pepper string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

func hasScope(scopes []string, required string) bool {
	for _, s := range scopes {
		if s == required {
			return true
		}
	}
	return false
}

func Digest(secret, pepper string) string {
	return digest(secret, pepper)
}

func PrefixOf(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])[:prefixLen]
}
