package auth

import (
	"errors"
	"testing"

	"github.com/xh-polaris/psych-core-api/biz/conf"
)

const testPepper = "unit-test-pepper"

func makeKeys() []conf.OpenApiKey {
	active := "UnitTestSecretKey0123456789abcdef"
	revoked := "RevokedSecretKey0123456789"
	return []conf.OpenApiKey{
		{Prefix: PrefixOf(active), Digest: Digest(active, testPepper), Status: "active", Scopes: []string{"psych:chat"}},
		{Prefix: PrefixOf(revoked), Digest: Digest(revoked, testPepper), Status: "revoked", Scopes: []string{"psych:chat"}},
	}
}

func tokenFor(secret string) string {
	return "sk_live_" + PrefixOf(secret) + "_" + secret
}

func TestVerifyAcceptsActiveKeyWithScope(t *testing.T) {
	secret := "UnitTestSecretKey0123456789abcdef"
	key, err := Verify(testPepper, makeKeys(), tokenFor(secret), "psych:chat")
	if err != nil {
		t.Fatalf("want pass, got %v", err)
	}
	if key.Prefix != PrefixOf(secret) {
		t.Errorf("prefix = %q, want %q", key.Prefix, PrefixOf(secret))
	}
}

func TestVerifyFailsClosed(t *testing.T) {
	secret := "UnitTestSecretKey0123456789abcdef"
	token := tokenFor(secret)
	if _, err := Verify("", makeKeys(), token, "psych:chat"); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty pepper: want ErrInvalid, got %v", err)
	}
	if _, err := Verify(testPepper, nil, token, "psych:chat"); !errors.Is(err, ErrInvalid) {
		t.Errorf("no keys: want ErrInvalid, got %v", err)
	}
}

func TestVerifyRejectsWrongOrMalformedTokens(t *testing.T) {
	cases := []string{
		"",
		"sk_live_",
		"not-a-key",
		"sk_live_short_x",
		"sk_live_00000000_" + "WrongSecretKey0123456789abcdef", // 未知 prefix
	}
	for _, token := range cases {
		if _, err := Verify(testPepper, makeKeys(), token, "psych:chat"); !errors.Is(err, ErrInvalid) {
			t.Errorf("token %q: want ErrInvalid, got %v", token, err)
		}
	}
	// 前缀正确但随机部分错误。
	if _, err := Verify(testPepper, makeKeys(), "sk_live_"+PrefixOf("UnitTestSecretKey0123456789abcdef")+"_WrongSecretKey0123456789", "psych:chat"); !errors.Is(err, ErrInvalid) {
		t.Errorf("wrong secret: want ErrInvalid, got %v", err)
	}
}

func TestVerifyRejectsRevokedAndMissingScope(t *testing.T) {
	if _, err := Verify(testPepper, makeKeys(), tokenFor("RevokedSecretKey0123456789"), "psych:chat"); !errors.Is(err, ErrRevoked) {
		t.Errorf("want ErrRevoked, got %v", err)
	}
	secret := "ScopedKeyWithoutChatScope0123456789"
	keys := []conf.OpenApiKey{
		{Prefix: PrefixOf(secret), Digest: Digest(secret, testPepper), Status: "active", Scopes: []string{"psych:report"}},
	}
	if _, err := Verify(testPepper, keys, tokenFor(secret), "psych:chat"); !errors.Is(err, ErrInsufficientScope) {
		t.Errorf("want ErrInsufficientScope, got %v", err)
	}
}
