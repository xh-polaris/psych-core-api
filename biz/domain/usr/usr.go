package usr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/synapse"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var _ IUserDomainSVC = (*UserDomainSVC)(nil)

type IUserDomainSVC interface {
	SignInByPhone(ctx context.Context, authType, phone, unitId, verify string) (*user.User, error)
	SignInByEmail(ctx context.Context, authType, email, unitId, verify string) (*user.User, error)
	SignInByCode(ctx context.Context, authType, studentID, unitId, verify string) (*user.User, error)
	ValidateCreateUserInput(input CreateUserInput) error
	CreateUser(ctx context.Context, input CreateUserInput) (*user.User, *unit.Unit, error)
	ValidatePasswordResetTarget(ctx context.Context, unitID string) error
	UpdatePassword(ctx context.Context, userId, newPassword string) error
	SendVerifyCode(ctx context.Context, authType, authId, cause string) error
}

// CreateUserInput 是创建心理用户所需的业务输入。
// 它刻意不依赖 HTTP/Proto DTO，使账号创建规则可由任何入口复用。
type CreateUserInput struct {
	UnitID     string
	Password   string
	Email      *string
	Phone      *string
	Code       *string
	CodeType   int
	Name       string
	Birth      int64
	Gender     int
	EnrollYear int
	Role       int
	Class      int
	CreateTime int64
	UpdateTime int64
}

type UserDomainSVC struct {
	UsrMapper  user.IMongoMapper
	UnitMapper unit.IMongoMapper
	Synp4bCli  synapse.Client
}

var UserDomainSet = wire.NewSet(
	wire.Struct(new(UserDomainSVC), "*"),
	wire.Bind(new(IUserDomainSVC), new(*UserDomainSVC)),
)

func (u *UserDomainSVC) SignInByPhone(ctx context.Context, authType, phone, unitId, verify string) (*user.User, error) {
	unitId, err := u.resolveLoginUnitID(ctx, phone, unitId)
	if err != nil {
		return nil, err
	}
	// 验证中台账号
	synpResp, err := u.Synp4bCli.Login(ctx, authType, phone, unitId, verify)
	if err != nil {
		return nil, err
	}

	// 查询本地账号
	pu, err := u.findPsychUser(ctx, synpResp)
	if err != nil || pu == nil {
		return nil, err
	}

	return pu, nil
}

func (u *UserDomainSVC) SignInByEmail(ctx context.Context, authType, email, unitId, verify string) (*user.User, error) {
	unitId, err := u.resolveLoginUnitID(ctx, email, unitId)
	if err != nil {
		return nil, err
	}
	// 验证中台账号
	synpResp, err := u.Synp4bCli.Login(ctx, authType, email, unitId, verify)
	if err != nil {
		return nil, err
	}

	// 查询本地账号
	pu, err := u.findPsychUser(ctx, synpResp)
	if err != nil || pu == nil {
		return nil, err
	}

	return pu, nil
}

func (u *UserDomainSVC) SignInByCode(ctx context.Context, authType, studentID, unitId, verify string) (*user.User, error) {
	unitId, err := u.resolveLoginUnitID(ctx, studentID, unitId)
	if err != nil {
		return nil, err
	}
	synpResp, err := u.Synp4bCli.Login(ctx, authType, studentID, unitId, verify)
	if err != nil {
		return nil, err
	}

	// 查询本地账号
	pu, err := u.findPsychUser(ctx, synpResp)
	if err != nil || pu == nil {
		return nil, err
	}

	return pu, nil
}

// resolveLoginUnitID 兼容不再由前端提交单位 ID 的管理端登录。
// 管理员账号在本地用户表中已绑定单位，因此可以先按账号反查单位，再将单位
// ID 作为 Synapse ToB 登录的内部参数传递；客户端无需感知或填写该字段。
func (u *UserDomainSVC) resolveLoginUnitID(ctx context.Context, authID, unitID string) (string, error) {
	if unitID != "" {
		return unitID, nil
	}

	localUser, err := u.UsrMapper.FindAdminByCode(ctx, authID, nil)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// 继续让 Synapse 校验账号，保持历史错误语义。
			return "", nil
		}
		return "", err
	}
	if localUser == nil || localUser.UnitID.IsZero() {
		return "", nil
	}
	return localUser.UnitID.Hex(), nil
}

func (u *UserDomainSVC) CreateUser(ctx context.Context, input CreateUserInput) (*user.User, *unit.Unit, error) {
	if err := u.ValidateCreateUserInput(input); err != nil {
		return nil, nil, err
	}

	// 账号创建需要同时保证中台单位和本地心理单位有效，这是一条跨系统的一致性规则。
	exists, err := u.synapseUnitExists(ctx, input.UnitID)
	if err != nil || !exists {
		return nil, nil, errorx.WrapByCode(err, errno.ErrCreateUser, errorx.KV("field", "单位不存在"))
	}
	pUnit, err := u.findActivePsychUnit(ctx, input.UnitID)
	if err != nil {
		return nil, nil, err
	}

	psychUser := buildPsychUser(input)

	// 创建 basicUser。encryptType 为 0 表示传入明文密码，中台使用 bcrypt 加密后存储。
	bu, err := u.Synp4bCli.CreateBasicUser(ctx, input.UnitID, valueOf(input.Code), valueOf(input.Phone), valueOf(input.Email), input.Password, 0)
	if err != nil {
		return nil, nil, err
	}

	// 创建psychUser
	oid, _ := bson.ObjectIDFromHex(bu.BasicUserID)
	psychUser.ID = oid

	// 校验重复创建
	if old, err := u.UsrMapper.FindOneById(ctx, oid); old != nil {
		return nil, nil, errorx.New(errno.ErrCreateUser, errorx.KV("field", "用户已存在"))
	} else if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil, err
	}

	err = u.UsrMapper.Insert(ctx, psychUser)
	if err != nil {
		logs.Error("[user mapper] insert new user failed", errorx.ErrorWithoutStack(err))
		return nil, nil, errorx.New(errno.ErrCreateUser, errorx.KV("field", "存储用户失败"))
	}

	return psychUser, pUnit, nil
}

func (u *UserDomainSVC) UpdatePassword(ctx context.Context, userId, newPassword string) error {
	// TODO
	return errorx.New(errno.ErrUnImplement)
}

// ValidatePasswordResetTarget 确认密码重置所针对的单位同时存在于中台和本地，
// 且本地单位处于启用状态。
func (u *UserDomainSVC) ValidatePasswordResetTarget(ctx context.Context, unitID string) error {
	exists, err := u.synapseUnitExists(ctx, unitID)
	if err != nil || !exists {
		return errorx.New(errno.ErrNotFound, errorx.KV("field", fmt.Sprintf("指定单位[id=%s]", unitID)))
	}
	_, err = u.findActivePsychUnit(ctx, unitID)
	return err
}

// 查询psychUser
func (u *UserDomainSVC) findPsychUser(ctx context.Context, synpResp *synapse.LoginResult) (*user.User, error) {
	// 查询psych用户
	Oids, err := util.ObjectIDsFromHex(synpResp.BasicUserID, synpResp.UnitID)
	if err != nil {
		logs.Error("[user domain] get basic user objectID from hex failed")
		return nil, err
	}

	usr, err := u.UsrMapper.FindOneById(ctx, Oids[0])
	if errors.Is(err, mongo.ErrNoDocuments) {
		logs.Errorf("[user mapper] psych user [id=%s] not found", synpResp.BasicUserID)
	} else if err != nil {
		logs.Error("[user mapper] FindOneById failed")
		return nil, err
	}

	return usr, err
}

func (u *UserDomainSVC) SendVerifyCode(ctx context.Context, authType, authId, cause string) error {
	// 验证码只能发送给已经建立中台与本地映射的心理用户。
	ok, err := u.UsrMapper.ExistsByCode(ctx, authId)
	if !ok {
		return errorx.New(errno.ErrNotFound, errorx.KV("field", fmt.Sprintf("指定用户[code=%s]", authId)))
	}
	if err != nil {
		return errorx.WrapByCode(err, errno.ErrSendVerifyCode)
	}

	err = u.Synp4bCli.SendVerifyCode(ctx, authType, authId, cause)
	if err != nil {
		return errorx.WrapByCode(err, errno.ErrSendVerifyCode)
	}

	return nil
}

func (u *UserDomainSVC) synapseUnitExists(ctx context.Context, unitID string) (bool, error) {
	synapseUnit, err := u.Synp4bCli.GetUnit(ctx, unitID)
	if err != nil {
		return false, err
	}
	return synapseUnit != nil, nil
}

func (u *UserDomainSVC) findActivePsychUnit(ctx context.Context, unitID string) (*unit.Unit, error) {
	unitOID, err := bson.ObjectIDFromHex(unitID)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "单位ID"))
	}

	psychUnit, err := u.UnitMapper.FindOneById(ctx, unitOID)
	if psychUnit == nil || errors.Is(err, mongo.ErrNoDocuments) || psychUnit.Status != enum.UnitStatusActive {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", fmt.Sprintf("指定单位[id=%s]", unitID)))
	}
	if err != nil {
		logs.Errorf("find unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}

	return psychUnit, nil
}

func (u *UserDomainSVC) ValidateCreateUserInput(input CreateUserInput) error {
	if input.UnitID == "" {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}
	if input.Code == nil && input.Phone == nil && input.Email == nil {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "验证码/手机号/邮箱不能全为空"))
	}
	if input.Phone == nil && input.Password == "" {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "密码"))
	}
	if input.Name == "" {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "姓名"))
	}
	if input.EnrollYear == 0 {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "入学年份"))
	}
	if input.Class == 0 {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "班级"))
	}
	if input.CodeType == 0 {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "code类型"))
	}
	if input.Role == 0 {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "角色"))
	}
	if input.Role != enum.UserRoleStudent &&
		input.Role != enum.UserRoleTeacher &&
		input.Role != enum.UserRoleClassTeacher &&
		input.Role != enum.UserRoleUnitAdmin {
		return errorx.New(errno.ErrInvalidParams, errorx.KV("field", "用户角色不合法"))
	}
	return nil
}

func buildPsychUser(input CreateUserInput) *user.User {
	unitID, _ := bson.ObjectIDFromHex(input.UnitID)
	gender := input.Gender
	if gender != enum.UserGenderMale && gender != enum.UserGenderFemale {
		gender = enum.UserGenderOther
	}

	// 同时传入 phone、email、code 时，心理用户展示标识优先级为 phone > email > studentID。
	code, codeType := "", 0
	if valueOf(input.Code) != "" {
		code, codeType = valueOf(input.Code), enum.UserCodeTypeStudentID
	}
	if valueOf(input.Email) != "" {
		code, codeType = valueOf(input.Email), enum.UserCodeTypeEmail
	}
	if valueOf(input.Phone) != "" {
		code, codeType = valueOf(input.Phone), enum.UserCodeTypePhone
	}

	var birth time.Time
	if input.Birth != 0 {
		birth = time.Unix(input.Birth, 0)
	}
	now := time.Now()
	createTime, updateTime := now, now
	if input.CreateTime != 0 {
		createTime = time.Unix(input.CreateTime, 0)
	}
	if input.UpdateTime != 0 {
		updateTime = time.Unix(input.UpdateTime, 0)
	}

	return &user.User{
		CodeType: codeType, Code: code, UnitID: unitID, Name: input.Name,
		Birth: birth, Gender: gender, RiskLevel: enum.UserRiskLevelLow, Status: enum.UserStatusActive,
		EnrollYear: input.EnrollYear, Role: input.Role, Class: input.Class,
		CreateTime: createTime, UpdateTime: updateTime,
	}
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
