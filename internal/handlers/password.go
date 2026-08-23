package handlers

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	minPasswordLen     = 8
	maxPasswordBytes   = 72 // bcrypt 有效输入上限，超过会静默截断，必须显式拒绝
)

// validatePasswordStrength 校验密码强度：
// - 8~72 字节（bcrypt 上限）
// - 至少一个大写、一个小写、一个数字、一个特殊字符
func validatePasswordStrength(password string) error {
	if len([]byte(password)) < minPasswordLen {
		return fmt.Errorf("密码长度不能少于 %d 位", minPasswordLen)
	}
	if len([]byte(password)) > maxPasswordBytes {
		return fmt.Errorf("密码过长，不能超过 %d 字节（UTF-8 中文约 %d 个字符）", maxPasswordBytes, maxPasswordBytes/3)
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	var missing []string
	if !hasUpper {
		missing = append(missing, "大写字母")
	}
	if !hasLower {
		missing = append(missing, "小写字母")
	}
	if !hasDigit {
		missing = append(missing, "数字")
	}
	if !hasSpecial {
		missing = append(missing, "特殊字符")
	}

	if len(missing) > 0 {
		return fmt.Errorf("密码须包含%s", strings.Join(missing, "、"))
	}

	return nil
}
