package user

import (
	"time"

	"github.com/xh-polaris/psych-core-api/biz/cst"
	"go.mongodb.org/mongo-driver/v2/bson"
	"time"
)

var (
	RiskLevelStoI = map[string]int32{cst.High: 1, cst.Medium: 2, cst.Low: 3, cst.Normal: 4}
	RiskLevelItoS = map[int32]string{1: cst.High, 2: cst.Medium, 3: cst.Low, 4: cst.Normal}
)

type User struct {
	ID         bson.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	CodeType   int            `json:"codeType,omitempty" bson:"codeType,omitempty"` // Phone | StudentID
	Code       string         `json:"code,omitempty" bson:"code,omitempty"`
	Password   string         `json:"password,omitempty" bson:"password,omitempty"`
	UnitID     bson.ObjectID  `json:"unitId,omitempty" bson:"unitId,omitempty"`
	Name       string         `json:"name,omitempty" bson:"name,omitempty"`
	Birth      time.Time      `json:"birth,omitempty" bson:"birth,omitempty"`
	Gender     int            `json:"gender,omitempty" bson:"gender,omitempty"`
	RiskLevel  int            `json:"riskLevel,omitempty" bson:"riskLevel,omitempty"`
	Status     int            `json:"status,omitempty" bson:"status,omitempty"`
	EnrollYear int32          `json:"enrollYear,omitempty" bson:"enrollYear,omitempty"`
	Grade      int32          `json:"grade,omitempty" bson:"grade,omitempty"`
	Class      int32          `json:"class,omitempty" bson:"class,omitempty"`
	Options    map[string]any `json:"option,omitempty" bson:"option,omitempty"`
	CreateTime time.Time      `json:"createTime,omitempty" bson:"createTime,omitempty"`
	UpdateTime time.Time      `json:"updateTime,omitempty" bson:"updateTime,omitempty"`
	DeleteTime time.Time      `json:"deleteTime,omitempty" bson:"deleteTime,omitempty"`
}
