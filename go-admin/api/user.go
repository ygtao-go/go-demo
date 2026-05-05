package api

import (
	"encoding/json"
	"fmt"
	"go-admin/config"
	"go-admin/model"
	"go-admin/pkg/response"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ================= 请求结构 =================

type LoginReq struct {
	Username string `json:"username" binding:"required,min=2,max=20"`
	Password string `json:"password" binding:"required,min=6,max=20"`
}

type RegisterReq struct {
	Username string `json:"username" binding:"required,min=2,max=20"`
	Password string `json:"password" binding:"required,min=6,max=20"`
}

// ================= 注册 =================

func Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}

	var count int64
	if err := config.DB.Model(&model.User{}).
		Where("username = ?", req.Username).
		Count(&count).Error; err != nil {
		response.Fail(c, 500, "数据库错误")
		return
	}

	if count > 0 {
		response.Fail(c, 400, "用户名已存在")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Fail(c, 500, "密码加密失败")
		return
	}

	user := model.User{
		Username: req.Username,
		Password: string(hash),
	}

	if err := config.DB.Create(&user).Error; err != nil {
		response.Fail(c, 500, "注册失败")
		return
	}

	response.Success(c, "注册成功")
}

// ================= 登录 =================

func Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	cacheKey := "user:" + req.Username

	// ===== Redis 缓存 =====
	if cached, err := config.RedisClient.Get(c, cacheKey).Result(); err == nil {
		var user model.User
		if json.Unmarshal([]byte(cached), &user) == nil {
			if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) == nil {
				token, _ := GenerateToken(user.ID)
				response.Success(c, token)
				return
			}
		}
	}

	// ===== 数据库 =====
	var user model.User
	if err := config.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		response.Fail(c, 400, "用户不存在")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		response.Fail(c, 400, "密码错误")
		return
	}

	// 写缓存
	if data, err := json.Marshal(user); err == nil {
		config.RedisClient.Set(c, cacheKey, data, 24*time.Hour)
	}

	token, err := GenerateToken(user.ID)
	if err != nil {
		response.Fail(c, 500, "token生成失败")
		return
	}

	response.Success(c, token)
}

// ================= 用户信息 =================

func GetUserInfo(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		response.Fail(c, 401, "未登录")
		return
	}

	var user model.User
	if err := config.DB.First(&user, userId).Error; err != nil {
		response.Fail(c, 404, "用户不存在")
		return
	}

	user.Password = ""
	response.Success(c, user)
}

// ================= 退出登录 =================

func Logout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	config.RedisClient.Set(c, "blacklist:"+token, "1", 24*time.Hour)
	response.Success(c, "退出成功")
}

// ================= 修改密码 =================

type UpdatePasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

func UpdatePassword(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		response.Fail(c, 401, "未登录")
		return
	}

	var req UpdatePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	var user model.User
	if err := config.DB.First(&user, userId).Error; err != nil {
		response.Fail(c, 404, "用户不存在")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)) != nil {
		response.Fail(c, 400, "旧密码错误")
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	user.Password = string(hash)

	config.DB.Save(&user)

	response.Success(c, "修改成功")
}

// ================= 用户列表 =================

func UserList(c *gin.Context) {
	var users []model.User
	var total int64

	page := 1
	pageSize := 10
	offset := (page - 1) * pageSize

	query := config.DB.Model(&model.User{})
	query.Count(&total)

	if err := query.Limit(pageSize).Offset(offset).Find(&users).Error; err != nil {
		response.Fail(c, 500, "查询失败")
		return
	}

	response.Success(c, gin.H{
		"list":  users,
		"total": total,
	})
}

// ================= 编辑用户（修复你报错的关键） =================

func EditUser(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Username string `json:"username"`
		Status   int    `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	var user model.User
	if err := config.DB.First(&user, id).Error; err != nil {
		response.Fail(c, 404, "用户不存在")
		return
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Status != 0 {
		user.Status = req.Status
	}

	if err := config.DB.Save(&user).Error; err != nil {
		response.Fail(c, 500, "更新失败")
		return
	}

	response.Success(c, "更新成功")
}

// ================= 删除用户 =================

func DeleteUser(c *gin.Context) {
	id := c.Param("id")

	if err := config.DB.Delete(&model.User{}, id).Error; err != nil {
		response.Fail(c, 500, "删除失败")
		return
	}

	response.Success(c, "删除成功")
}

// ================= 状态切换 =================

func SwitchStatus(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Status int `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	config.DB.Model(&model.User{}).
		Where("id = ?", id).
		Update("status", req.Status)

	response.Success(c, "状态更新成功")
}

// ================= 示例 Token =================

func GenerateToken(userID uint) (string, error) {
	return fmt.Sprintf("token-%d", userID), nil
}
