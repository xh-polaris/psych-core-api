package enum

// auth type
const (
	AuthTypePassword = 0
	AuthTypeCode     = 1
)

var AuthTypeAtoI = map[string]int{
	"password": AuthTypePassword,
	"code":     AuthTypeCode,
}

var AuthTypeItoA = map[int]string{
	AuthTypePassword: "password",
	AuthTypeCode:     "code",
}
