package utils

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt/v5"
	"github.com/xh-polaris/psych-core-api/biz/domain/usr"
	"github.com/xh-polaris/psych-core-api/biz/infra/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/consts"
	"github.com/xh-polaris/psych-pkg/httpx"
)

func GenerateJwt(claims map[string]any) (string, error) {
	mapClaims := jwt.MapClaims{}
	for k, v := range claims {
		mapClaims[k] = v
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, mapClaims)
	privateKey, err := jwt.ParseECPrivateKeyFromPEM([]byte(conf.GetConfig().Auth.SecretKey))
	if err != nil {
		return "", err
	}
	return token.SignedString(privateKey)
}

func ParseJwt(jwtStr string, options ...jwt.ParserOption) (jwt.MapClaims, error) {
	token, err := jwt.Parse(jwtStr, func(_ *jwt.Token) (interface{}, error) {
		return jwt.ParseECPublicKeyFromPEM([]byte(conf.GetConfig().Auth.PublicKey))
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

func ExtraUserMeta(ctx context.Context) (m *usr.Meta, err error) {
	var meta usr.Meta
	var c *app.RequestContext
	var claims jwt.MapClaims
	if c, err = httpx.ExtractContext(ctx); err != nil {
		return nil, err
	}
	if claims, err = ParseJwt(string(c.GetHeader("Authorization"))); err != nil {
		return nil, err
	}
	meta.UserId = claims[consts.UserId].(string)
	meta.UnitId = claims[consts.UnitId].(string)
	meta.StudentId = claims[consts.StudentId].(string)
	meta.Strong = claims[consts.Strong].(bool)
	return &meta, nil
}
