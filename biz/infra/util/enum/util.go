package enum

func ParseStatus(status string) (int, bool) {
	val, ok := statusMap[status]
	return val, ok
}

func ParseGender(gender string) (int, bool) {
	val, ok := genderMap[gender]
	return val, ok
}

func ParseCodeType(codeType string) (int, bool) {
	val, ok := codeTypeMap[codeType]
	return val, ok
}

func ParseConfigType(configType string) (int, bool) {
	val, ok := configTypeMap[configType]
	return val, ok
}

func GetStatus(status int) (string, bool) {
	val, ok := statusMapReverse[status]
	return val, ok
}

func GetGender(gender int) (string, bool) {
	val, ok := genderMapReverse[gender]
	return val, ok
}

func GetCodeType(codeType int) (string, bool) {
	val, ok := codeTypeMapReverse[codeType]
	return val, ok
}

func GetConfigType(configType int) (string, bool) {
	val, ok := configTypeMapReverse[configType]
	return val, ok
}
