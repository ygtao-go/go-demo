package config

import (
	"fmt"
	"go-admin/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 导出全局 DB 变量
var DB *gorm.DB

func InitDB() *gorm.DB {
	dsn := "root:123456@tcp(127.0.0.1:3307)/go_admin?charset=utf8mb4&parseTime=True&loc=Local"
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("数据库连接失败: %v", err))
	}

	// 自动创建表
	err = DB.AutoMigrate(&model.User{})
	if err != nil {
		panic(fmt.Sprintf("表创建失败: %v", err))
	}

	fmt.Println("✅ MySQL 连接成功，用户表已就绪！")
	return DB
}
