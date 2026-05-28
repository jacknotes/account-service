package handlers

import (
	"fmt"
	"strings"
	"unicode"
)

// validatePasswordStrength validates password complexity requirements:
// - Minimum 8 characters
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one digit
// - At least one special character
// Returns nil if valid, or an error with a Chinese message describing the requirement.
func validatePasswordStrength(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("密码长度不能少于 8 位")
	}

	var (
		hasUpper bool
		hasLower bool
		hasDigit bool
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
