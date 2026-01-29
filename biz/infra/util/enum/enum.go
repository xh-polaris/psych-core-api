package enum

// status
const (
	Active  = 0
	Deleted = 1
)

// gender
const (
	Unknown = 0
	Male    = 1
	Female  = 2
)

// code type
const (
	CodeTypePhone     = 0
	CodeTypeStudentID = 1
)

// config type
const (
	ConfigTypeChain   = 0
	ConfigTypeEnd2End = 1
)

var statusMap = map[string]int{
	"active":  Active,
	"deleted": Deleted,
}

var genderMap = map[string]int{
	"unknown": Unknown,
	"male":    Male,
	"female":  Female,
}

var codeTypeMap = map[string]int{
	"phone":     CodeTypePhone,
	"studentId": CodeTypeStudentID,
}

var configTypeMap = map[string]int{
	"chain":   ConfigTypeChain,
	"end2end": ConfigTypeEnd2End,
}

var statusMapReverse = map[int]string{
	Active:  "active",
	Deleted: "deleted",
}

var genderMapReverse = map[int]string{
	Unknown: "unknown",
	Male:    "male",
	Female:  "female",
}

var codeTypeMapReverse = map[int]string{
	CodeTypePhone:     "phone",
	CodeTypeStudentID: "studentId",
}

var configTypeMapReverse = map[int]string{
	ConfigTypeChain:   "chain",
	ConfigTypeEnd2End: "end2end",
}
