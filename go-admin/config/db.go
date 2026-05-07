package config

import (
	"fmt"
	"go-admin/model"
	"os" // 用于读取环境变量

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 导出全局 DB 变量
var DB *gorm.DB

func InitDB() *gorm.DB {
	// 从环境变量读取数据库连接信息
	host := os.Getenv("DB_HOST")     // 数据库 IP 或域名
	port := os.Getenv("DB_PORT")     // 数据库端口
	user := os.Getenv("DB_USER")     // 用户名
	pass := os.Getenv("DB_PASSWORD") // 密码
	name := os.Getenv("DB_NAME")     // 数据库名

	// 拼接 DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, pass, host, port, name)

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
