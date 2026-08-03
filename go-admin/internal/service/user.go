package service

import (
	"errors"
	"log"
	"time"

	"go-admin/internal/repository"
	"go-admin/model"
	"go-admin/pkg/metrics"
	"go-admin/utils"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// ErrUserNotFound 用户不存在的统一错误（缓存穿透 / 击穿路径共用）
var ErrUserNotFound = errors.New("用户不存在")

// userSingleflight 用户读取单飞：同一 username 同一时间只允许一次"完整缓存回填流程"，
// 其余并发请求等待并共享结果（防缓存击穿）。
// 整个流程（正常缓存 → none 缓存 → MySQL → 回填）都在 flight 内，
// 即使请求在飞行中途才到达，也会命中回填后的缓存，保证 MySQL 至多查询一次。
var userSingleflight singleflight.Group

// userDBFlight 登录专用 DB 加载单飞：登录必须拿到 DB 中的真实密码哈希，
// 与读取路径（可能命中不含密码的缓存）隔离，避免并发时误用缓存结果导致登录失败。
var userDBFlight singleflight.Group

// ==================== 缓存防护：穿透 / 击穿 / 雪崩 ====================

// GetUserByUsernameProtected 带缓存保护的按用户名查询：
//  1. 命中正常缓存（user:<username>）→ 直接返回（不含密码）
//  2. 命中不存在缓存（user:none:<username>）→ 直接返回"用户不存在"，不打 MySQL（防缓存穿透）
//  3. 未命中 → 查 MySQL，同一 username 并发时仅一次 DB 查询（防缓存击穿）
//  4. 回填缓存：存在写正常缓存（随机 TTL，防缓存雪崩），不存在写 none 缓存（5 分钟）
func GetUserByUsernameProtected(username string) (*model.User, error) {
	v, err, _ := userSingleflight.Do(username, func() (interface{}, error) {
		// 1. 正常缓存
		if cached, err := repository.GetCachedUser(username); err == nil {
			return cached.ToModelUser(), nil
		}

		// 2. none 缓存：存在则直接返回不存在
		if exists, err := repository.GetNoneCachedUser(username); err == nil && exists {
			return nil, ErrUserNotFound
		}

		// 3. MySQL 查询
		user, err := repository.GetUserByUsername(username)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 4. 不存在 → 写 none 缓存（防缓存穿透）
				if werr := repository.SetNoneCachedUser(username); werr != nil {
					log.Printf("写入 none 缓存失败 username=%s err=%v", username, werr)
				}
				return nil, ErrUserNotFound
			}
			return nil, err
		}

		// 5. 存在 → 写正常缓存（随机 TTL 防雪崩）；缓存失败不影响主流程
		if cerr := repository.CacheUser(user, repository.UserCacheBaseTTL); cerr != nil {
			log.Printf("写入用户缓存失败 username=%s err=%v", username, cerr)
		}
		return user, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*model.User), nil
}

// loadUserFromDB 登录专用：singleflight 保护下查询 MySQL 并回填缓存。
// 返回的 user 带 DB 中的真实密码哈希（登录密码校验必须依赖 DB 数据，缓存不含密码）。
func loadUserFromDB(username string) (*model.User, error) {
	v, err, _ := userDBFlight.Do(username, func() (interface{}, error) {
		user, err := repository.GetUserByUsername(username)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// MySQL 不存在 → 写 none 缓存（防缓存穿透）
				if werr := repository.SetNoneCachedUser(username); werr != nil {
					log.Printf("写入 none 缓存失败 username=%s err=%v", username, werr)
				}
				return nil, ErrUserNotFound
			}
			return nil, err
		}

		// 存在 → 写正常缓存（随机 TTL 防雪崩）；缓存失败不影响主流程
		if cerr := repository.CacheUser(user, repository.UserCacheBaseTTL); cerr != nil {
			log.Printf("写入用户缓存失败 username=%s err=%v", username, cerr)
		}
		return user, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*model.User), nil
}

func Login(username, password string) (map[string]interface{}, error) {
	// 1. 缓存穿透保护：命中 none 缓存直接返回"用户不存在"，不打 MySQL
	if exists, err := repository.GetNoneCachedUser(username); err == nil && exists {
		return nil, ErrUserNotFound
	}

	// 2. 查数据库（singleflight 单飞，同一 username 并发仅一次 MySQL 查询）
	//    密码校验必须走 DB：Redis 缓存不存储密码、不用于密码校验
	user, err := loadUserFromDB(username)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// 3. bcrypt 校验（使用数据库中的密码哈希）
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errors.New("密码错误")
	}

	// 4. 生成双 Token
	return generateTokenResult(user.ID)
}

// generateTokenResult 生成双 Token 并持久化 refresh
func generateTokenResult(userId uint) (map[string]interface{}, error) {
	accessToken, accessJTI, err := utils.GenerateAccessToken(userId)
	if err != nil {
		return nil, errors.New("token生成失败")
	}

	refreshToken, refreshJTI, err := utils.GenerateRefreshToken(userId)
	if err != nil {
		return nil, errors.New("token生成失败")
	}

	// 同步持久化 refresh jti 到 Redis（7 天有效期）
	// 必须同步：确保返回的 refresh token 真实可用，且登出删除后不会被后台任务重新写回
	if err := repository.SaveRefreshJTI(userId, refreshJTI, 7*24*time.Hour); err != nil {
		return nil, errors.New("登录失败，请稍后重试")
	}

	return map[string]interface{}{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"accessJTI":    accessJTI,
		"refreshJTI":   refreshJTI,
	}, nil
}

// RefreshToken 刷新 Token（Rotation 机制）
// 通过命名返回值 + defer 统一上报 refresh 成功/失败业务指标，不改动任何返回语义。
func RefreshToken(refreshTokenStr string) (result map[string]interface{}, err error) {
	// 业务指标：refresh 成功/失败次数
	defer func() {
		metrics.RecordRefresh(err == nil)
	}()

	// 1. 解析 refresh token
	claims, err := utils.ParseRefreshToken(refreshTokenStr)
	if err != nil {
		return nil, errors.New("refresh token无效")
	}

	// 2. 原子消费旧 refresh jti（Lua：检查+删除一步完成，消除并发刷新窗口）
	// 消费成功（true）：旧 token 已删除，且仅本请求获得换新资格
	// 消费失败（false）：旧 token 已被消费/撤销，直接拒绝
	consumed, err := repository.ConsumeRefreshJTI(claims.UserId, claims.JTI)
	if err != nil {
		return nil, errors.New("token刷新失败")
	}
	if !consumed {
		return nil, errors.New("refresh token已过期或已被撤销")
	}

	// 4. 生成新的双 Token
	accessToken, accessJTI, err := utils.GenerateAccessToken(claims.UserId)
	if err != nil {
		return nil, errors.New("token生成失败")
	}

	refreshToken, refreshJTI, err := utils.GenerateRefreshToken(claims.UserId)
	if err != nil {
		return nil, errors.New("token生成失败")
	}

	// 5. 同步持久化新 refresh jti（旧 jti 已删除，新 jti 必须写入成功，避免用户被登出）
	if err := repository.SaveRefreshJTI(claims.UserId, refreshJTI, 7*24*time.Hour); err != nil {
		return nil, errors.New("token刷新失败")
	}

	return map[string]interface{}{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"accessJTI":    accessJTI,
		"refreshJTI":   refreshJTI,
	}, nil
}

func Register(username, password string) error {
	// 1. 检查用户名唯一性
	_, err := repository.GetUserByUsername(username)
	if err == nil {
		return errors.New("用户名已存在")
	}

	// 2. 密码加密
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码加密失败")
	}

	// 3. 创建用户
	user := &model.User{
		Username: username,
		Password: string(hash),
	}
	if err := repository.CreateUser(user); err != nil {
		return err
	}

	// 4. 清除该用户名可能残留的 none 缓存，避免新注册用户被误判为"不存在"
	if err := repository.DeleteNoneCachedUser(username); err != nil {
		log.Printf("删除 none 缓存失败 username=%s err=%v", username, err)
	}
	return nil
}

func GetUserInfo(userID uint) (*model.User, error) {
	user, err := repository.GetUserByID(userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	user.Password = ""
	return user, nil
}

func Logout(accessToken, refreshToken string) error {
	// 1. 解析 access token 并撤销 jti
	if claims, err := utils.ParseAccessToken(accessToken); err == nil {
		// 计算剩余有效期作为黑名单 TTL
		remaining := time.Until(claims.ExpiresAt.Time)
		if remaining > 0 {
			repository.AddJTIBlacklist(claims.JTI, remaining)
		}
	}

	// 2. 解析 refresh token 并撤销 jti
	// 同时删除 rt:jti:<hash> 与 rt:user:<userId> 中的 jti（双向索引一致）
	if claims, err := utils.ParseRefreshToken(refreshToken); err == nil {
		repository.DeleteRefreshJTI(claims.UserId, claims.JTI)
		// 同时加入黑名单（防止黑名单检查阶段通过）
		repository.AddJTIBlacklist(claims.JTI, 7*24*time.Hour)
	}

	return nil
}

func UpdatePassword(userID uint, oldPassword, newPassword string) error {
	user, err := repository.GetUserByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return errors.New("旧密码错误")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码加密失败")
	}

	if err := repository.UpdateUserPassword(userID, string(hash)); err != nil {
		return err
	}

	// 修改密码成功后必须删除用户缓存，禁止保留旧缓存
	if err := repository.DeleteCachedUser(user.Username); err != nil {
		log.Printf("删除用户缓存失败 username=%s err=%v", user.Username, err)
	}
	return nil
}

func ListUsers(page, pageSize int) ([]model.User, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}
	return repository.ListUsers(page, pageSize)
}

func EditUser(id uint, username string, status int) error {
	user, err := repository.GetUserByID(id)
	if err != nil {
		return errors.New("用户不存在")
	}

	oldUsername := user.Username
	if username != "" {
		user.Username = username
	}
	if status != 0 {
		user.Status = status
	}

	if err := repository.UpdateUser(user); err != nil {
		return err
	}

	// 缓存失效：旧用户名缓存必须删除（改名后旧 key 不能残留）
	if err := repository.DeleteCachedUser(oldUsername); err != nil {
		log.Printf("删除旧用户缓存失败 username=%s err=%v", oldUsername, err)
	}
	// 写入最新数据缓存：用户名变化时写入新 key，未改名时等价刷新
	// 使用基础 TTL（24h），CacheUser 内部会自动叠加 ±1h 随机偏移防雪崩
	if err := repository.CacheUser(user, repository.UserCacheBaseTTL); err != nil {
		log.Printf("写入用户缓存失败 username=%s err=%v", user.Username, err)
	}
	return nil
}

func DeleteUser(id uint) error {
	// 删除前先取用户名，用于缓存失效
	user, err := repository.GetUserByID(id)
	if err != nil {
		return errors.New("用户不存在")
	}

	if err := repository.DeleteUser(id); err != nil {
		return err
	}

	// 删除用户缓存
	if err := repository.DeleteCachedUser(user.Username); err != nil {
		log.Printf("删除用户缓存失败 username=%s err=%v", user.Username, err)
	}

	// 清理该用户所有 refresh token 索引（rt:jti:<hash> 与 rt:user:<id>），避免 Redis 残留
	if err := repository.CleanUserRefreshTokens(id); err != nil {
		log.Printf("清理用户 refresh token 失败 userId=%d err=%v", id, err)
	}

	return nil
}

func SwitchStatus(id uint, status int) error {
	if status != 1 && status != 2 {
		return errors.New("状态值无效（1=正常, 2=禁用）")
	}

	// 需要用户名来删除缓存，先查询一次用户
	user, err := repository.GetUserByID(id)
	if err != nil {
		return errors.New("用户不存在")
	}

	if err := repository.UpdateUserStatus(id, status); err != nil {
		return err
	}

	// 修改状态成功后必须删除用户缓存
	if err := repository.DeleteCachedUser(user.Username); err != nil {
		log.Printf("删除用户缓存失败 username=%s err=%v", user.Username, err)
	}
	return nil
}
