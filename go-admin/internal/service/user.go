package service

import (
	"errors"
	"go-admin/internal/repository"
	"go-admin/utils"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func Login(username, password string) (string, error) {
	user, err := repository.GetUserByUsername(username)
	if err != nil {
		return "", errors.New("用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("密码错误")
	}

	token, err := utils.GenerateToken(user.ID)
	if err != nil {
		return "", err
	}

	// ✅ 加一个并发（加分点）
	go repository.CacheUser(user, 24*time.Hour)

	return token, nil
}
