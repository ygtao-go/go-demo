package repository

import (
	"context"
	"encoding/json"
	"go-admin/config"
	"go-admin/model"
	"time"
)

var ctx = context.Background()

func GetUserByUsername(username string) (*model.User, error) {
	var user model.User
	err := config.DB.Where("username = ?", username).First(&user).Error
	return &user, err
}

func CacheUser(user *model.User, ttl time.Duration) {
	key := "user:" + user.Username

	data, err := json.Marshal(user)
	if err != nil {
		return
	}

	config.RedisClient.Set(ctx, key, data, ttl)
}
