package alert

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/sms_alert"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/sms"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ────────── 基础 mock 内嵌，统一处理未使用的方法 ──────────

type baseMapper[T any] struct{ t *testing.T }

func (m *baseMapper[T]) fatal(method string) { m.t.Fatalf("unexpected call to %s", method) }

func (m *baseMapper[T]) FindOneByFields(ctx context.Context, filter bson.M) (*T, error) {
	m.fatal("FindOneByFields")
	return nil, nil
}
func (m *baseMapper[T]) FindAllByFields(ctx context.Context, filter bson.M) ([]*T, error) {
	m.fatal("FindAllByFields")
	return nil, nil
}
func (m *baseMapper[T]) FindManyWithOption(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*T, error) {
	m.fatal("FindManyWithOption")
	return nil, nil
}
func (m *baseMapper[T]) UpdateFields(ctx context.Context, id bson.ObjectID, update bson.M) error {
	m.fatal("UpdateFields")
	return nil
}
func (m *baseMapper[T]) ExistsByFields(ctx context.Context, filter bson.M) (bool, error) {
	m.fatal("ExistsByFields")
	return false, nil
}
func (m *baseMapper[T]) CountByPeriod(ctx context.Context, start, end time.Time) (int32, error) {
	m.fatal("CountByPeriod")
	return 0, nil
}
func (m *baseMapper[T]) CountByFields(ctx context.Context, filter bson.M) (int32, error) {
	m.fatal("CountByFields")
	return 0, nil
}

// ────────── mock user.IMongoMapper ──────────

type mockUserMapper struct {
	baseMapper[user.User]
	findOneByIdResult      *user.User
	findOneByIdErr         error
	findClassTeacherResult *user.User
	findClassTeacherErr    error
}

func (m *mockUserMapper) FindOneById(ctx context.Context, id bson.ObjectID) (*user.User, error) {
	return m.findOneByIdResult, m.findOneByIdErr
}
func (m *mockUserMapper) Insert(ctx context.Context, data *user.User) error {
	m.fatal("Insert")
	return nil
}

func (m *mockUserMapper) FindStudentByCode(ctx context.Context, code string, unitId bson.ObjectID) (*user.User, error) {
	m.fatal("FindStudentByCode")
	return nil, nil
}
func (m *mockUserMapper) FindAdminByCode(ctx context.Context, code string, unitId *bson.ObjectID) (*user.User, error) {
	m.fatal("FindAdminByCode")
	return nil, nil
}
func (m *mockUserMapper) CountStudents(ctx context.Context, unitId bson.ObjectID) (int32, error) {
	m.fatal("CountStudents")
	return 0, nil
}
func (m *mockUserMapper) CountStudentsByPeriod(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error) {
	m.fatal("CountStudentsByPeriod")
	return 0, nil
}
func (m *mockUserMapper) CountHighRiskStudents(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) (int32, error) {
	m.fatal("CountHighRiskStudents")
	return 0, nil
}
func (m *mockUserMapper) FindOneByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (*user.User, error) {
	m.fatal("FindOneByCodeAndUnitID")
	return nil, nil
}
func (m *mockUserMapper) FindOneByCodeAndRole(ctx context.Context, code string, role int) (*user.User, error) {
	m.fatal("FindOneByCodeAndRole")
	return nil, nil
}
func (m *mockUserMapper) ExistsByCodeAndUnitID(ctx context.Context, code string, unitId bson.ObjectID) (bool, error) {
	m.fatal("ExistsByCodeAndUnitID")
	return false, nil
}
func (m *mockUserMapper) FindAllByUnitID(ctx context.Context, unitId bson.ObjectID) ([]*user.User, error) {
	m.fatal("FindAllByUnitID")
	return nil, nil
}
func (m *mockUserMapper) FindManyByUnitIDWithFilter(ctx context.Context, unitId bson.ObjectID, grade, class *int32) ([]*user.User, error) {
	m.fatal("FindManyByUnitIDWithFilter")
	return nil, nil
}
func (m *mockUserMapper) BatchFindByIDs(ctx context.Context, userIds []bson.ObjectID) (map[bson.ObjectID]*user.User, error) {
	m.fatal("BatchFindByIDs")
	return nil, nil
}
func (m *mockUserMapper) CountByClasses(ctx context.Context, unitId bson.ObjectID, startGrade int, grade, class []int32) ([]*user.ClassStatResult, error) {
	m.fatal("CountByClasses")
	return nil, nil
}
func (m *mockUserMapper) RiskDistributionStats(ctx context.Context, unitId *bson.ObjectID, start, end time.Time) ([]*user.RiskStat, error) {
	m.fatal("RiskDistributionStats")
	return nil, nil
}
func (m *mockUserMapper) FindUnitClassTeachers(ctx context.Context, unitId bson.ObjectID, startGrade int) (user.ClassTeachers, error) {
	m.fatal("FindUnitClassTeachers")
	return nil, nil
}
func (m *mockUserMapper) ExistsClassTeacher(ctx context.Context, unitId bson.ObjectID, grade, class int) (bool, error) {
	m.fatal("ExistsClassTeacher")
	return false, nil
}
func (m *mockUserMapper) ExistsByCode(ctx context.Context, code string) (bool, error) {
	m.fatal("ExistsByCode")
	return false, nil
}
func (m *mockUserMapper) CountHighRiskByGrade(ctx context.Context, unitId bson.ObjectID, startGrade int) (map[int32]int32, int32, error) {
	m.fatal("CountHighRiskByGrade")
	return nil, 0, nil
}
func (m *mockUserMapper) CountHighRiskByGradeAndClasses(ctx context.Context, startGrade int, enrollYears, classes []int32) (map[int32]int32, int32, error) {
	m.fatal("CountHighRiskByGradeAndClasses")
	return nil, 0, nil
}
func (m *mockUserMapper) GetClassTeacherBoundClasses(ctx context.Context, userId bson.ObjectID) ([]user.ClassInfo, error) {
	m.fatal("GetClassTeacherBoundClasses")
	return nil, nil
}
func (m *mockUserMapper) CountStudentsByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32) (int32, error) {
	m.fatal("CountStudentsByClassList")
	return 0, nil
}
func (m *mockUserMapper) CountStudentsByPeriodAndClassList(ctx context.Context, unitId *bson.ObjectID, grades, classes []int32, start, end time.Time) (int32, error) {
	m.fatal("CountStudentsByPeriodAndClassList")
	return 0, nil
}
func (m *mockUserMapper) CountHighRiskStudentsByClassList(ctx context.Context, grades, classes []int32, start, end time.Time) (int32, error) {
	m.fatal("CountHighRiskStudentsByClassList")
	return 0, nil
}
func (m *mockUserMapper) FindManyByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32) ([]*user.User, error) {
	m.fatal("FindManyByClassList")
	return nil, nil
}
func (m *mockUserMapper) GetRiskDistributionByClassList(ctx context.Context, unitId bson.ObjectID, grades, classes []int32, start, end time.Time) ([]*user.RiskStat, error) {
	m.fatal("GetRiskDistributionByClassList")
	return nil, nil
}
func (m *mockUserMapper) ListUsers(ctx context.Context, opts *user.ListUserOptions) ([]*user.User, int64, error) {
	m.fatal("ListUsers")
	return nil, 0, nil
}
func (m *mockUserMapper) FindClassTeacherOfStudent(ctx context.Context, unitId bson.ObjectID, enrollYear, class int) (*user.User, error) {
	return m.findClassTeacherResult, m.findClassTeacherErr
}

var _ user.IMongoMapper = (*mockUserMapper)(nil)

// ────────── mock unit.IMongoMapper ──────────

type mockUnitMapper struct {
	baseMapper[unit.Unit]
	findOneByIdResult *unit.Unit
	findOneByIdErr    error
}

func (m *mockUnitMapper) Insert(ctx context.Context, data *unit.Unit) error {
	m.fatal("Insert")
	return nil
}
func (m *mockUnitMapper) FindOneById(ctx context.Context, id bson.ObjectID) (*unit.Unit, error) {
	return m.findOneByIdResult, m.findOneByIdErr
}
func (m *mockUnitMapper) Count(ctx context.Context) (int32, error) { m.fatal("Count"); return 0, nil }
func (m *mockUnitMapper) FindAll(ctx context.Context) ([]*unit.Unit, error) {
	m.fatal("FindAll")
	return nil, nil
}
func (m *mockUnitMapper) FindOneByURI(ctx context.Context, uri string) (*unit.Unit, error) {
	m.fatal("FindOneByURI")
	return nil, nil
}
func (m *mockUnitMapper) FindOneByContact(ctx context.Context, contact string) (*unit.Unit, error) {
	m.fatal("FindOneByContact")
	return nil, nil
}

var _ unit.IMongoMapper = (*mockUnitMapper)(nil)

// ────────── mock config.IMongoMapper ──────────

type mockConfigMapper struct {
	baseMapper[config.Config]
	findByUnitIDResult *config.Config
	findByUnitIDErr    error
}

func (m *mockConfigMapper) Insert(ctx context.Context, data *config.Config) error {
	m.fatal("Insert")
	return nil
}
func (m *mockConfigMapper) FindOneById(ctx context.Context, id bson.ObjectID) (*config.Config, error) {
	m.fatal("FindOneById")
	return nil, nil
}
func (m *mockConfigMapper) FindOneByUnitID(ctx context.Context, unitID bson.ObjectID) (*config.Config, error) {
	return m.findByUnitIDResult, m.findByUnitIDErr
}
func (m *mockConfigMapper) PushCharacter(ctx context.Context, configID bson.ObjectID, ch *config.Character) error {
	m.fatal("PushCharacter")
	return nil
}
func (m *mockConfigMapper) SetCharacter(ctx context.Context, configID bson.ObjectID, chID bson.ObjectID, update bson.M) error {
	m.fatal("SetCharacter")
	return nil
}
func (m *mockConfigMapper) SetCharacterStatus(ctx context.Context, configID bson.ObjectID, chID bson.ObjectID, status int) error {
	m.fatal("SetCharacterStatus")
	return nil
}

var _ config.IMongoMapper = (*mockConfigMapper)(nil)

// ────────── mock sms_alert.IMongoMapper ──────────

type mockSmsAlertMapper struct {
	baseMapper[sms_alert.SmsAlert]
	countTodayResult int32
	countTodayErr    error
	insertErr        error
	inserted         []*sms_alert.SmsAlert
}

func (m *mockSmsAlertMapper) FindOneById(ctx context.Context, id bson.ObjectID) (*sms_alert.SmsAlert, error) {
	m.fatal("FindOneById")
	return nil, nil
}
func (m *mockSmsAlertMapper) Insert(ctx context.Context, data *sms_alert.SmsAlert) error {
	if m.insertErr != nil {
		return m.insertErr
	}
	m.inserted = append(m.inserted, data)
	return nil
}
func (m *mockSmsAlertMapper) CountTodayByUserId(ctx context.Context, userId bson.ObjectID, start, end time.Time) (int32, error) {
	return m.countTodayResult, m.countTodayErr
}

var _ sms_alert.IMongoMapper = (*mockSmsAlertMapper)(nil)

// ────────── mock cache.Cmdable ──────────

type mockCache struct {
	setNXOk  bool
	setNXErr error
}

func (m *mockCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) cache.BoolCmd {
	return &mockBoolCmd{ok: m.setNXOk, err: m.setNXErr}
}
func (m *mockCache) Pipeline() cache.Pipeliner { return nil }
func (m *mockCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) cache.StatusCmd {
	return nil
}
func (m *mockCache) Get(ctx context.Context, key string) cache.StringCmd              { return nil }
func (m *mockCache) IncrBy(ctx context.Context, key string, value int64) cache.IntCmd { return nil }
func (m *mockCache) Incr(ctx context.Context, key string) cache.IntCmd                { return nil }
func (m *mockCache) HSet(ctx context.Context, key string, values ...interface{}) cache.IntCmd {
	return nil
}
func (m *mockCache) HGetAll(ctx context.Context, key string) cache.MapStringStringCmd { return nil }
func (m *mockCache) Del(ctx context.Context, keys ...string) cache.IntCmd             { return nil }
func (m *mockCache) Exists(ctx context.Context, keys ...string) cache.IntCmd          { return nil }
func (m *mockCache) Expire(ctx context.Context, key string, expiration time.Duration) cache.BoolCmd {
	return nil
}
func (m *mockCache) LIndex(ctx context.Context, key string, index int64) cache.StringCmd { return nil }
func (m *mockCache) LPush(ctx context.Context, key string, values ...interface{}) cache.IntCmd {
	return nil
}
func (m *mockCache) RPush(ctx context.Context, key string, values ...interface{}) cache.IntCmd {
	return nil
}
func (m *mockCache) LSet(ctx context.Context, key string, index int64, value interface{}) cache.StatusCmd {
	return nil
}
func (m *mockCache) LPop(ctx context.Context, key string) cache.StringCmd { return nil }
func (m *mockCache) LRange(ctx context.Context, key string, start, stop int64) cache.StringSliceCmd {
	return nil
}
func (m *mockCache) Eval(ctx context.Context, script string, keys []string, args ...interface{}) cache.Cmd {
	return nil
}

type mockBoolCmd struct {
	ok  bool
	err error
}

func (c *mockBoolCmd) Err() error            { return c.err }
func (c *mockBoolCmd) Result() (bool, error) { return c.ok, c.err }

var _ cache.Cmdable = (*mockCache)(nil)

// ────────── mock sms.Provider ──────────

type mockSmsProvider struct {
	calledWithCause  string
	calledWithPhone  []string
	calledWithParams [][]string
	sendErr          error
}

func (m *mockSmsProvider) Send(ctx context.Context, cause, phone string, params []string) error {
	m.calledWithCause = cause
	m.calledWithPhone = append(m.calledWithPhone, phone)
	m.calledWithParams = append(m.calledWithParams, params)
	return m.sendErr
}

var _ sms.Provider = (*mockSmsProvider)(nil)

// ────────── tests ──────────

func TestSendSmsAlert(t *testing.T) {
	unitId := bson.NewObjectID()
	studentId := bson.NewObjectID()
	teacherId := bson.NewObjectID()
	convId := bson.NewObjectID()

	startGrade := 1
	enrollYear := 2025

	userMapper := &mockUserMapper{
		findOneByIdResult: &user.User{
			ID: studentId, UnitID: unitId, Name: "小明",
			EnrollYear: enrollYear, Class: 1,
			Role: enum.UserRoleStudent, CodeType: enum.UserCodeTypeStudentID,
			Status: enum.UserStatusActive,
		},
		findClassTeacherResult: &user.User{
			ID: teacherId, UnitID: unitId, Name: "王老师",
			Code: "13800138000", CodeType: enum.UserCodeTypePhone,
			Role: enum.UserRoleClassTeacher, Status: enum.UserStatusActive,
		},
		baseMapper: baseMapper[user.User]{t: t},
	}

	unitMapper := &mockUnitMapper{
		findOneByIdResult: &unit.Unit{
			ID: unitId, Name: "测试小学", StartGrade: startGrade,
			Status: enum.UnitStatusActive,
		},
		baseMapper: baseMapper[unit.Unit]{t: t},
	}

	configMapper := &mockConfigMapper{
		findByUnitIDResult: &config.Config{AlertPhone: []string{"13900139000"}},
		baseMapper:         baseMapper[config.Config]{t: t},
	}

	smsAlertMapper := &mockSmsAlertMapper{baseMapper: baseMapper[sms_alert.SmsAlert]{t: t}}
	smsProvider := &mockSmsProvider{}

	oldSmsMgr, oldAlertMgr := sms.Mgr, Mgr
	defer func() { sms.Mgr = oldSmsMgr; Mgr = oldAlertMgr }()
	sms.Mgr = smsProvider
	Mgr = &AlertManager{
		cache: &mockCache{setNXOk: true}, userMapper: userMapper,
		configMapper: configMapper, unitMapper: unitMapper,
		smsAlertMapper: smsAlertMapper,
	}

	err := Mgr.Send(context.Background(), unitId, studentId, convId)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if smsProvider.calledWithCause != "alert" {
		t.Errorf("cause: got %q, want %q", smsProvider.calledWithCause, "alert")
	}
	if len(smsProvider.calledWithPhone) == 0 {
		t.Fatal("no SMS send calls")
	}
	t.Logf("sent to %d phone(s): %v", len(smsProvider.calledWithPhone), smsProvider.calledWithPhone)

	for i, p := range smsProvider.calledWithParams {
		t.Logf("  phone=%s params=%v", smsProvider.calledWithPhone[i], p)
		if len(p) != 3 {
			t.Errorf("param count: got %d, want 3", len(p))
			continue
		}
		if p[0] != "小明" {
			t.Errorf("param[0]: got %q, want %q", p[0], "小明")
		}
		if p[1] != "1" {
			t.Errorf("param[1] grade: got %q, want %q", p[1], "1")
		}
		if p[2] != "1" {
			t.Errorf("param[2] class: got %q, want %q", p[2], "1")
		}
	}

	if len(smsAlertMapper.inserted) != 1 {
		t.Fatalf("sms_alert records: got %d, want 1", len(smsAlertMapper.inserted))
	}
	r := smsAlertMapper.inserted[0]
	if r.UnitID != unitId || r.UserID != studentId || r.ConvID != convId {
		t.Errorf("record mismatch: unit=%s user=%s conv=%s", r.UnitID.Hex(), r.UserID.Hex(), r.ConvID.Hex())
	}
	t.Logf("sms_alert: recipients=%v", r.Recipients)
}

func TestSendSmsAlert_RateLimited(t *testing.T) {
	unitId := bson.NewObjectID()
	studentId := bson.NewObjectID()
	convId := bson.NewObjectID()

	userMapper := &mockUserMapper{
		findOneByIdResult: &user.User{
			ID: studentId, UnitID: unitId, Name: "小明",
			EnrollYear: 2025, Class: 1,
			Role: enum.UserRoleStudent, Status: enum.UserStatusActive,
		},
		baseMapper: baseMapper[user.User]{t: t},
	}
	unitMapper := &mockUnitMapper{
		findOneByIdResult: &unit.Unit{ID: unitId, StartGrade: 1, Status: enum.UnitStatusActive},
		baseMapper:        baseMapper[unit.Unit]{t: t},
	}
	configMapper := &mockConfigMapper{
		findByUnitIDResult: &config.Config{AlertPhone: []string{"13900139000"}},
		baseMapper:         baseMapper[config.Config]{t: t},
	}
	smsAlertMapper := &mockSmsAlertMapper{baseMapper: baseMapper[sms_alert.SmsAlert]{t: t}}
	smsProvider := &mockSmsProvider{}

	oldSmsMgr, oldAlertMgr := sms.Mgr, Mgr
	defer func() { sms.Mgr = oldSmsMgr; Mgr = oldAlertMgr }()
	sms.Mgr = smsProvider
	Mgr = &AlertManager{
		cache: &mockCache{setNXOk: false}, userMapper: userMapper,
		configMapper: configMapper, unitMapper: unitMapper,
		smsAlertMapper: smsAlertMapper,
	}

	err := Mgr.Send(context.Background(), unitId, studentId, convId)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if len(smsProvider.calledWithPhone) > 0 {
		t.Error("expected no SMS due to rate limit")
	}
	if len(smsAlertMapper.inserted) > 0 {
		t.Error("expected no sms_alert record due to rate limit")
	}
	t.Log("rate limit correctly blocked SMS")
}

func TestSendSmsAlert_NoRecipients(t *testing.T) {
	unitId := bson.NewObjectID()
	studentId := bson.NewObjectID()
	convId := bson.NewObjectID()

	userMapper := &mockUserMapper{
		findOneByIdResult: &user.User{
			ID: studentId, UnitID: unitId, Name: "小明",
			EnrollYear: 2025, Class: 1,
			Role: enum.UserRoleStudent, Status: enum.UserStatusActive,
		},
		findClassTeacherResult: nil,
		findClassTeacherErr:    fmt.Errorf("not found"),
		baseMapper:             baseMapper[user.User]{t: t},
	}
	unitMapper := &mockUnitMapper{
		findOneByIdResult: &unit.Unit{ID: unitId, StartGrade: 1, Status: enum.UnitStatusActive},
		baseMapper:        baseMapper[unit.Unit]{t: t},
	}
	configMapper := &mockConfigMapper{
		findByUnitIDResult: &config.Config{AlertPhone: nil},
		baseMapper:         baseMapper[config.Config]{t: t},
	}
	smsAlertMapper := &mockSmsAlertMapper{baseMapper: baseMapper[sms_alert.SmsAlert]{t: t}}
	smsProvider := &mockSmsProvider{}

	oldSmsMgr, oldAlertMgr := sms.Mgr, Mgr
	defer func() { sms.Mgr = oldSmsMgr; Mgr = oldAlertMgr }()
	sms.Mgr = smsProvider
	Mgr = &AlertManager{
		cache: &mockCache{setNXOk: true}, userMapper: userMapper,
		configMapper: configMapper, unitMapper: unitMapper,
		smsAlertMapper: smsAlertMapper,
	}

	err := Mgr.Send(context.Background(), unitId, studentId, convId)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if len(smsProvider.calledWithPhone) > 0 {
		t.Error("expected no SMS (no recipients)")
	}
	t.Log("no recipients — correctly skipped")
}
