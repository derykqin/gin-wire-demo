package router

import (
	"regexp"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func registerValidator() {
	// 注册自定义验证器
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("mobile", mobileValidator)
	}
}

// 自定义手机号验证函数
func mobileValidator(fl validator.FieldLevel) bool {
	// 正则匹配中国大陆手机号 (1开头，第二位3-9，共11位)
	reg := `^1[3-9]\d{9}$`
	return regexp.MustCompile(reg).MatchString(fl.Field().String())
}
