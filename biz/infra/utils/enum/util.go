package enum

func ParseAuthType(authType string) (int, bool) {
	val, ok := authTypeMap[authType]
	return val, ok
}

func GetAuthType(authType int) (string, bool) {
	val, ok := authTypeMapReverse[authType]
	return val, ok
}
