package enum

// auth type
const (
	authTypePassword = 0
	authTypeCode     = 1
)

var authTypeMap = map[string]int{
	"password": authTypePassword,
	"code":     authTypeCode,
}

var authTypeMapReverse = map[int]string{
	authTypePassword: "password",
	authTypeCode:     "code",
}
