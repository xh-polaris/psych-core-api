package util

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt/v5"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/types/enum"
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
func ExtraUserMeta(ctx context.Context) (m *Meta, err error) {
	var meta Meta
	var c *app.RequestContext
	var claims jwt.MapClaims
	if c, err = httpx.ExtractContext(ctx); err != nil {
		return nil, errorx.New(errno.ErrUnAuth)
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

// Meta 是jwt的claim负载，包含用户基础信息和权限等级
type Meta struct {
	UserId string `json:"userId"`
	UnitId string `json:"unitId;omitempty"`
	Code   string `json:"code;omitempty"`
	Role   int    `json:"role"` // 权限等级 (学生用户、老师、班主任、单位管理、超管)
}

func (usrMeta Meta) HasTeacherAuth() bool {
	return usrMeta.Role >= enum.UserRoleTeacher
}

func (usrMeta Meta) HasClassTeacherAuth() bool {
	return usrMeta.Role >= enum.UserRoleClassTeacher
}

func (usrMeta Meta) HasUnitAdminAuth(unitId string) bool {
	return usrMeta.HasSuperAdminAuth() || (usrMeta.Role == enum.UserRoleUnitAdmin && usrMeta.UnitId == unitId)
}

func (usrMeta Meta) HasSuperAdminAuth() bool {
	return usrMeta.Role >= enum.UserRoleSuperAdmin
}
