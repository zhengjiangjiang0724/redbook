package dao

import (
	"redbook/model"

	"gorm.io/gorm"
)

type UserDAO struct {
	db *gorm.DB
}

// NewUserDAO 创建一个新的 UserDAO 实例
func NewUserDAO(db *gorm.DB) *UserDAO {
	return &UserDAO{db: db}
}

// CreateUser 创建新用户
func (dao *UserDAO) CreateUser(user *model.User) error {
	return dao.db.Create(user).Error
}

// FindByMobile 根据手机号查询用户
func (dao *UserDAO) FindByMobile(mobile string) (*model.User, error) {
	var user model.User
	err := dao.db.Where("mobile = ?", mobile).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername 根据用户名获取用户
func (dao *UserDAO) GetByUsername(username string) (*model.User, error) {
	var user model.User
	err := dao.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
