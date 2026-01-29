package reg

import "regexp"

// CheckMobile 检验手机号
func CheckMobile(phone string) bool {
	if len(phone) == 0 {
		return false
	}

	// 国内手机号规则
	domesticPattern := "^1[345789]\\d{9}$"

	// 国际手机号规则: +开头，后面跟 6-15 位数字
	// 这里假设国际手机号总长度 7~16 位（包括 +）
	internationalPattern := "^\\+\\d{6,15}$"

	var reg *regexp.Regexp

	if phone[0] == '1' {
		reg = regexp.MustCompile(domesticPattern)
	} else if phone[0] == '+' {
		reg = regexp.MustCompile(internationalPattern)
	} else {
		return false
	}

	return reg.MatchString(phone)
}
