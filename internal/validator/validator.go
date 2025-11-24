package validator

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

// IsMobile 是一个自定义的校验函数，用于验证手机号格式
func IsMobile(fl validator.FieldLevel) bool {
	mobile := fl.Field().String()
	// 简单的手机号正则表达式
	re := regexp.MustCompile(`^1[3-9]\d{9}$`)
	return re.MatchString(mobile)
}
