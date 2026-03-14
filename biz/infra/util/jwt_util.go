package util

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt/v5"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/usr"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/types/errno"
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
		return nil, errorx.New(errno.ErrJWTPrase)
	}
	return token.Claims.(jwt.MapClaims), nil
}

// ExtraUserMeta 从ctx中提取出userId
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
	meta.UserId = claims[cst.JsonUserID].(string)
	meta.UnitId = claims[cst.JsonUnitID].(string)
	meta.Code = claims[cst.JsonCode].(string)
	meta.Role = int(claims[cst.JsonRole].(float64))

	return &meta, nil
}
