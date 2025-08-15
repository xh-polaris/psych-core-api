package utils

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/xh-polaris/psych-core-api/biz/infra/consts"
)

func GenerateJwt(key any, claims map[string]any) (string, error) {
	mapClaims := jwt.MapClaims{}
	for k, v := range claims {
		mapClaims[k] = v
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)
	signedString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}
	return signedString, nil
}

func ParseJwt(key any, jwtStr string, options ...jwt.ParserOption) (jwt.MapClaims, error) {
	token, err := jwt.Parse(jwtStr, func(token *jwt.Token) (interface{}, error) {
		return key, nil
	}, options...)
	if err != nil {
		return nil, err
	}
	// 校验 Claims 对象是否有效，基于 exp（过期时间），nbf（不早于），iat（签发时间）等进行判断（如果有这些声明的话）。
	if !token.Valid {
		return nil, consts.JwtParseErr
	}
	return token.Claims.(jwt.MapClaims), nil
}
