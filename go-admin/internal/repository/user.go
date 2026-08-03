package repository

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"

	"go-admin/config"
	"go-admin/model"
	"go-admin/utils"
)

// ==================== User CRUD ====================

func GetUserByUsername(username string) (*model.User, error) {
	var user model.User
	err := config.DB.Where("username = ?", username).First(&user).Error
	return &user, err
}

func GetUserByID(id uint) (*model.User, error) {
	var user model.User
	err := config.DB.First(&user, id).Error
	return &user, err
}

func CreateUser(user *model.User) error {
	return config.DB.Create(user).Error
}

func ListUsers(page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	offset := (page - 1) * pageSize

	query := config.DB.Model(&model.User{})
	query.Count(&total)

	if err := query.Limit(pageSize).Offset(offset).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func UpdateUser(user *model.User) error {
	return config.DB.Save(user).Error
}

func DeleteUser(id uint) error {
	return config.DB.Delete(&model.User{}, id).Error
}

func UpdateUserStatus(id uint, status int) error {
	return config.DB.Model(&model.User{}).Where("id = ?", id).Update("status", status).Error
}

func UpdateUserPassword(id uint, hashedPassword string) error {
	return config.DB.Model(&model.User{}).Where("id = ?", id).Update("password", hashedPassword).Error
}

// ==================== 缓存 TTL 策略（防缓存雪崩） ====================

const (
	// UserCacheBaseTTL 用户缓存基础 TTL：24 小时
	UserCacheBaseTTL = 24 * time.Hour
	// UserCacheJitter 用户缓存 TTL 随机偏移：±1 小时（最终 TTL 落在 23h ~ 25h）
	// 随机化后同一批缓存不会在同一时刻集体过期，避免缓存雪崩
	UserCacheJitter = time.Hour
	// UserNoneCacheTTL 用户不存在缓存 TTL：5 分钟（防缓存穿透）
	UserNoneCacheTTL = 5 * time.Minute
)

// jitteredTTL 在基础 TTL 上叠加 ±jitter 的随机偏移
func jitteredTTL(base, jitter time.Duration) time.Duration {
	offset := time.Duration(rand.Int64N(2*int64(jitter)+1)) - jitter
	return base + offset
}

// ==================== Redis Cache ====================

// CachedUser Redis 用户缓存结构
// 注意：刻意不包含 Password 字段 —— Redis 缓存禁止存储任何密码信息
type CachedUser struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToModelUser 转换为 model.User（密码字段留空 —— 缓存中本就不存储密码）
func (c *CachedUser) ToModelUser() *model.User {
	return &model.User{
		ID:        c.ID,
		Username:  c.Username,
		Status:    c.Status,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// CacheUser 同步写入用户基础信息缓存（不含密码，仅用于基础信息读取）
// TTL 随机化：在传入的基础 TTL 上叠加 ±1 小时随机偏移（基础 24h → 23h~25h），
// 防止大量缓存同时过期造成缓存雪崩
func CacheUser(user *model.User, baseTTL time.Duration) error {
	ttl := jitteredTTL(baseTTL, UserCacheJitter)
	key := config.RedisKey("user", user.Username)
	cached := CachedUser{
		ID:        user.ID,
		Username:  user.Username,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	ctx, cancel := config.RedisContext()
	defer cancel()
	return config.RedisClient.Set(ctx, key, data, ttl).Err()
}

// GetCachedUser 读取用户基础信息缓存（不含密码）
func GetCachedUser(username string) (*CachedUser, error) {
	key := config.RedisKey("user", username)
	ctx, cancel := config.RedisContext()
	defer cancel()
	data, err := config.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var user CachedUser
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetCacheTTL 读取用户缓存的剩余 TTL（观测 / 测试用）
func GetCacheTTL(username string) (time.Duration, error) {
	key := config.RedisKey("user", username)
	ctx, cancel := config.RedisContext()
	defer cancel()
	return config.RedisClient.TTL(ctx, key).Result()
}

// DeleteCachedUser 删除指定用户名的用户缓存（缓存失效）
func DeleteCachedUser(username string) error {
	key := config.RedisKey("user", username)
	ctx, cancel := config.RedisContext()
	defer cancel()
	return config.RedisClient.Del(ctx, key).Err()
}

// ==================== 缓存穿透保护（用户不存在缓存） ====================

// GetNoneCachedUser 检查用户不存在缓存是否存在。
// 存在表示该用户名近期查过 MySQL 且不存在，直接短路返回，避免无效查询打到 MySQL。
func GetNoneCachedUser(username string) (bool, error) {
	key := config.RedisKey("user", "none", username)
	ctx, cancel := config.RedisContext()
	defer cancel()
	n, err := config.RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// SetNoneCachedUser 写入用户不存在缓存（TTL 5 分钟，防缓存穿透）
func SetNoneCachedUser(username string) error {
	key := config.RedisKey("user", "none", username)
	ctx, cancel := config.RedisContext()
	defer cancel()
	return config.RedisClient.Set(ctx, key, "1", UserNoneCacheTTL).Err()
}

// DeleteNoneCachedUser 删除用户不存在缓存（注册新用户时调用，避免误判）
func DeleteNoneCachedUser(username string) error {
	key := config.RedisKey("user", "none", username)
	ctx, cancel := config.RedisContext()
	defer cancel()
	return config.RedisClient.Del(ctx, key).Err()
}

// ==================== JTI Blacklist（Redis） ====================

// AddJTIBlacklist 将 jti 加入黑名单（用 hash 缩短 key）
func AddJTIBlacklist(jti string, ttl time.Duration) error {
	key := config.RedisKey("bl", utils.JTIHash(jti))
	ctx, cancel := config.RedisContext()
	defer cancel()
	return config.RedisClient.Set(ctx, key, "1", ttl).Err()
}

// CheckJTIBlacklist 检查 jti 是否在黑名单中
func CheckJTIBlacklist(jti string) (bool, error) {
	key := config.RedisKey("bl", utils.JTIHash(jti))
	ctx, cancel := config.RedisContext()
	defer cancel()
	exists, err := config.RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// ==================== Refresh Token 管理 ====================

// SaveRefreshJTI 保存 refresh token 的 jti
func SaveRefreshJTI(userId uint, jti string, ttl time.Duration) error {
	// 支持按 userId 查询（用户维度）和按 jti 查询（精确撤销）
	jtiKey := config.RedisKey("rt", "jti", utils.JTIHash(jti))
	userKey := config.RedisKey("rt", "user", fmt.Sprintf("%d", userId))
	ctx, cancel := config.RedisContext()
	defer cancel()
	pipe := config.RedisClient.Pipeline()
	pipe.Set(ctx, jtiKey, userId, ttl)
	// 用户维度索引，便于用户注销时批量清理
	pipe.SAdd(ctx, userKey, utils.JTIHash(jti))
	pipe.Expire(ctx, userKey, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// DeleteRefreshJTI 删除 refresh jti（双向索引同步删除）
// 1. DEL rt:jti:<hash>        —— 使该 refresh token 立即失效
// 2. SREM rt:user:<userId> <hash> —— 同步移除用户维度索引成员，避免 rt:user 集合垃圾累积
// 两个操作放同一 Pipeline 同步执行，保证 rt:jti 与 rt:user 双向索引一致。
func DeleteRefreshJTI(userId uint, jti string) error {
	jtiKey := config.RedisKey("rt", "jti", utils.JTIHash(jti))
	userKey := config.RedisKey("rt", "user", fmt.Sprintf("%d", userId))
	ctx, cancel := config.RedisContext()
	defer cancel()
	pipe := config.RedisClient.Pipeline()
	pipe.Del(ctx, jtiKey)
	pipe.SRem(ctx, userKey, utils.JTIHash(jti))
	_, err := pipe.Exec(ctx)
	return err
}

// ConsumeRefreshJTI 原子消费 refresh jti（单次 Redis EVAL，消除「检查-删除」并发窗口）
// Lua 逻辑：
//  1. EXISTS rt:jti:<hash> —— 不存在则返回 0（已被消费/撤销）
//  2. 存在：DEL rt:jti:<hash> + SREM rt:user:<userId> <hash>，返回 1（消费成功）
//
// 并发下 Redis 串行执行脚本，同一 jti 只会被一个请求消费成功。
func ConsumeRefreshJTI(userId uint, jti string) (bool, error) {
	const consumeScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
    redis.call('DEL', KEYS[1])
    redis.call('SREM', KEYS[2], ARGV[1])
    return 1
end
return 0
`
	jtiKey := config.RedisKey("rt", "jti", utils.JTIHash(jti))
	userKey := config.RedisKey("rt", "user", fmt.Sprintf("%d", userId))
	ctx, cancel := config.RedisContext()
	defer cancel()

	res, err := config.RedisClient.Eval(ctx, consumeScript, []string{jtiKey, userKey}, utils.JTIHash(jti)).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// CleanUserRefreshTokens 清理用户所有 refresh token
func CleanUserRefreshTokens(userId uint) error {
	key := config.RedisKey("rt", "user", fmt.Sprintf("%d", userId))
	ctx, cancel := config.RedisContext()
	defer cancel()
	jtiHashes, err := config.RedisClient.SMembers(ctx, key).Result()
	if err != nil {
		return err
	}
	pipe := config.RedisClient.Pipeline()
	for _, jtiHash := range jtiHashes {
		pipe.Del(ctx, config.RedisKey("rt", "jti", jtiHash))
	}
	pipe.Del(ctx, key)
	_, err = pipe.Exec(ctx)
	return err
}
