// internal/model/user.go
package model

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username string `gorm:"size:255;not null;unique" json:"username"`
	Password string `gorm:"size:255;not null" json:"-"`
	Email    string `gorm:"size:255;unique" json:"email"`
}

// GetUserID 实现 jwt.Identity 接口
func (u *User) GetUserID() uint {
	return u.ID
}
