package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go-admin/router"

	"github.com/gin-gonic/gin"
)

// ============================================================
// 用户认证流程接口测试（第一阶段）
//
// 覆盖：
//   POST /api/user/register
//   POST /api/user/login
//   GET  /api/user/info
//   POST /api/user/refresh
//   POST /api/user/logout
//
// 测试重点：HTTP 状态码、response JSON、JWT 鉴权、
//           refresh token rotation、logout 后 refresh token 失效。
// ============================================================

// apiResult 标准响应信封 {"code":..., "msg":..., "data":...}
type apiResult struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// tokenData 登录/刷新返回的双 Token 结构
type tokenData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	AccessJTI    string `json:"accessJTI"`
	RefreshJTI   string `json:"refreshJTI"`
}

// newTestServer 构建测试用 gin 引擎。
// 仅挂载业务路由（router.InitRouter），不含 main 级的 Logger / RedisLimit / Recovery 中间件，
// 避免日志噪音与限流干扰测试结果。
func newTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	router.InitRouter(r)
	return r
}

// doJSON 构造并执行一次 JSON 请求。
func doJSON(t *testing.T, srv *gin.Engine, method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求体失败: %v", err)
		}
		buf.Write(b)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// decodeResult 解析标准响应信封。
func decodeResult(t *testing.T, w *httptest.ResponseRecorder) apiResult {
	t.Helper()
	var r apiResult
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v, body=%s", err, w.Body.String())
	}
	return r
}

// decodeTokens 从响应的 data 字段解析双 Token。
func decodeTokens(t *testing.T, data json.RawMessage) tokenData {
	t.Helper()
	var td tokenData
	if err := json.Unmarshal(data, &td); err != nil {
		t.Fatalf("解析 token 数据失败: %v (data=%s)", err, string(data))
	}
	if td.AccessToken == "" || td.RefreshToken == "" {
		t.Fatalf("token 数据不完整: %s", string(data))
	}
	return td
}

// registerUser 注册一个用户，失败则终止测试。
func registerUser(t *testing.T, srv *gin.Engine, username, password string) {
	t.Helper()
	w := doJSON(t, srv, http.MethodPost, "/api/user/register", map[string]string{
		"username": username,
		"password": password,
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("注册失败 HTTP=%d body=%s", w.Code, w.Body.String())
	}
	r := decodeResult(t, w)
	if r.Code != 0 {
		t.Fatalf("注册业务码失败: code=%d msg=%s", r.Code, r.Msg)
	}
}

// loginTokens 登录并返回双 Token，失败则终止测试。
func loginTokens(t *testing.T, srv *gin.Engine, username, password string) tokenData {
	t.Helper()
	w := doJSON(t, srv, http.MethodPost, "/api/user/login", map[string]string{
		"username": username,
		"password": password,
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("登录失败 HTTP=%d body=%s", w.Code, w.Body.String())
	}
	r := decodeResult(t, w)
	if r.Code != 0 {
		t.Fatalf("登录业务码失败: code=%d msg=%s", r.Code, r.Msg)
	}
	return decodeTokens(t, r.Data)
}

// uniqueUsername 生成带时间戳的唯一用户名，避免测试间数据污染。
func uniqueUsername(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), rand.IntN(100000))
}

const testPassword = "Test123456"

// ==================== POST /api/user/register ====================

func TestRegisterUser(t *testing.T) {
	srv := newTestServer()

	// 1. 正常注册 → HTTP 200 + code=0
	username := uniqueUsername("reg")
	registerUser(t, srv, username, testPassword)

	// 2. 重复注册同一用户名 → HTTP 400 + msg=用户名已存在
	w := doJSON(t, srv, http.MethodPost, "/api/user/register", map[string]string{
		"username": username,
		"password": testPassword,
	}, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("重复注册期望 HTTP 400，实际 %d", w.Code)
	}
	r := decodeResult(t, w)
	if r.Code != 400 {
		t.Errorf("重复注册期望业务码 400，实际 %d", r.Code)
	}
	if r.Msg != "用户名已存在" {
		t.Errorf("重复注册期望 msg=用户名已存在，实际 %q", r.Msg)
	}

	// 3. 缺失必填字段 → HTTP 400 + msg=参数错误
	w = doJSON(t, srv, http.MethodPost, "/api/user/register", map[string]string{
		"username": uniqueUsername("reg"),
	}, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("缺失密码注册期望 HTTP 400，实际 %d", w.Code)
	}
	r = decodeResult(t, w)
	if r.Msg != "参数错误" {
		t.Errorf("缺失密码注册期望 msg=参数错误，实际 %q", r.Msg)
	}
}

// ==================== POST /api/user/login ====================

func TestLoginUser(t *testing.T) {
	srv := newTestServer()
	username := uniqueUsername("login")
	registerUser(t, srv, username, testPassword)

	// 1. 正确密码 → HTTP 200 + 双 Token
	tokens := loginTokens(t, srv, username, testPassword)
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("登录未返回双 Token: %+v", tokens)
	}

	// 2. 错误密码 → HTTP 400 + msg=密码错误
	w := doJSON(t, srv, http.MethodPost, "/api/user/login", map[string]string{
		"username": username,
		"password": "WrongPass1",
	}, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("错误密码登录期望 HTTP 400，实际 %d", w.Code)
	}
	r := decodeResult(t, w)
	if r.Msg != "密码错误" {
		t.Errorf("错误密码登录期望 msg=密码错误，实际 %q", r.Msg)
	}

	// 3. 不存在的用户 → HTTP 400 + msg=用户不存在
	w = doJSON(t, srv, http.MethodPost, "/api/user/login", map[string]string{
		"username": uniqueUsername("nouser"),
		"password": testPassword,
	}, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("不存在用户登录期望 HTTP 400，实际 %d", w.Code)
	}
	r = decodeResult(t, w)
	if r.Msg != "用户不存在" {
		t.Errorf("不存在用户登录期望 msg=用户不存在，实际 %q", r.Msg)
	}
}

// ==================== GET /api/user/info（JWT 鉴权） ====================

func TestGetUserInfo(t *testing.T) {
	srv := newTestServer()
	username := uniqueUsername("info")
	registerUser(t, srv, username, testPassword)
	tokens := loginTokens(t, srv, username, testPassword)

	// 1. 无 Authorization 头 → HTTP 401
	w := doJSON(t, srv, http.MethodGet, "/api/user/info", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("无 token 期望 HTTP 401，实际 %d", w.Code)
	}

	// 2. 非法 token → HTTP 401
	w = doJSON(t, srv, http.MethodGet, "/api/user/info", nil, map[string]string{
		"Authorization": "Bearer not.a.valid.token",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("非法 token 期望 HTTP 401，实际 %d", w.Code)
	}

	// 3. 有效 access token → HTTP 200 + 用户信息（不含密码）
	w = doJSON(t, srv, http.MethodGet, "/api/user/info", nil, map[string]string{
		"Authorization": "Bearer " + tokens.AccessToken,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("有效 token 期望 HTTP 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	var info struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		Status   int    `json:"status"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(decodeResult(t, w).Data, &info); err != nil {
		t.Fatalf("解析用户信息失败: %v", err)
	}
	if info.Username != username {
		t.Errorf("期望 username=%s，实际 %s", username, info.Username)
	}
	if info.Password != "" {
		t.Errorf("用户信息泄漏密码字段！password=%q", info.Password)
	}
}

// ==================== POST /api/user/refresh（Rotation） ====================

func TestRefreshTokenRotation(t *testing.T) {
	srv := newTestServer()
	username := uniqueUsername("refresh")
	registerUser(t, srv, username, testPassword)
	old := loginTokens(t, srv, username, testPassword)

	// 1. 使用旧 refresh token 刷新 → HTTP 200 + 返回全新 Token 对
	w := doJSON(t, srv, http.MethodPost, "/api/user/refresh", map[string]string{
		"refreshToken": old.RefreshToken,
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("刷新期望 HTTP 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	r := decodeResult(t, w)
	if r.Code != 0 {
		t.Fatalf("刷新业务码失败: code=%d msg=%s", r.Code, r.Msg)
	}
	rotated := decodeTokens(t, r.Data)
	if rotated.RefreshToken == old.RefreshToken {
		t.Errorf("rotation 失效：新旧 refresh token 相同")
	}

	// 2. 旧 refresh token 复用 → HTTP 401（已被原子消费，rotation 生效）
	w = doJSON(t, srv, http.MethodPost, "/api/user/refresh", map[string]string{
		"refreshToken": old.RefreshToken,
	}, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("旧 refresh 复用期望 HTTP 401，实际 %d", w.Code)
	}
	r = decodeResult(t, w)
	if r.Msg != "refresh token已过期或已被撤销" {
		t.Errorf("旧 refresh 复用期望 msg=%q，实际 %q", "refresh token已过期或已被撤销", r.Msg)
	}

	// 3. 新 refresh token 仍可用 → HTTP 200
	w = doJSON(t, srv, http.MethodPost, "/api/user/refresh", map[string]string{
		"refreshToken": rotated.RefreshToken,
	}, nil)
	if w.Code != http.StatusOK {
		t.Errorf("新 refresh 再次刷新期望 HTTP 200，实际 %d body=%s", w.Code, w.Body.String())
	}

	// 4. 非法 refresh token → HTTP 401
	w = doJSON(t, srv, http.MethodPost, "/api/user/refresh", map[string]string{
		"refreshToken": "invalid-refresh-token",
	}, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("非法 refresh 期望 HTTP 401，实际 %d", w.Code)
	}
}

// ==================== POST /api/user/logout（登出后双 Token 失效） ====================

func TestLogoutInvalidatesTokens(t *testing.T) {
	srv := newTestServer()
	username := uniqueUsername("logout")
	registerUser(t, srv, username, testPassword)
	tokens := loginTokens(t, srv, username, testPassword)

	// 1. 登出 → HTTP 200 + data=退出成功
	w := doJSON(t, srv, http.MethodPost, "/api/user/logout", map[string]string{
		"refreshToken": tokens.RefreshToken,
	}, map[string]string{
		"Authorization": "Bearer " + tokens.AccessToken,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("登出期望 HTTP 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	r := decodeResult(t, w)
	if r.Code != 0 {
		t.Fatalf("登出业务码失败: code=%d msg=%s", r.Code, r.Msg)
	}
	if string(r.Data) != `"退出成功"` {
		t.Errorf("登出 data 期望 %q，实际 %s", "退出成功", string(r.Data))
	}

	// 2. 登出后 access token 已进黑名单 → GET /api/user/info → HTTP 401
	w = doJSON(t, srv, http.MethodGet, "/api/user/info", nil, map[string]string{
		"Authorization": "Bearer " + tokens.AccessToken,
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("登出后 access token 期望 HTTP 401，实际 %d", w.Code)
	}
	r = decodeResult(t, w)
	if r.Msg != "token已失效，请重新登录" {
		t.Errorf("登出后 access token 期望 msg=%q，实际 %q", "token已失效，请重新登录", r.Msg)
	}

	// 3. 登出后 refresh token 已删除/黑名单 → 刷新 → HTTP 401
	w = doJSON(t, srv, http.MethodPost, "/api/user/refresh", map[string]string{
		"refreshToken": tokens.RefreshToken,
	}, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("登出后 refresh token 期望 HTTP 401，实际 %d", w.Code)
	}
}

// ==================== 补充：登出缺少 refreshToken ====================

func TestLogoutRequiresRefreshToken(t *testing.T) {
	srv := newTestServer()
	username := uniqueUsername("logoutreq")
	registerUser(t, srv, username, testPassword)
	tokens := loginTokens(t, srv, username, testPassword)

	// 带有效 access token 但缺少 refreshToken → HTTP 400 参数错误
	w := doJSON(t, srv, http.MethodPost, "/api/user/logout", map[string]interface{}{}, map[string]string{
		"Authorization": "Bearer " + tokens.AccessToken,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("登出缺少 refreshToken 期望 HTTP 400，实际 %d", w.Code)
	}
	r := decodeResult(t, w)
	if r.Msg != "参数错误" {
		t.Errorf("登出缺少 refreshToken 期望 msg=参数错误，实际 %q", r.Msg)
	}
}

// ==================== 补充：access token 不能当作 refresh token（类型隔离） ====================

func TestAccessTokenCannotRefresh(t *testing.T) {
	srv := newTestServer()
	username := uniqueUsername("typechk")
	registerUser(t, srv, username, testPassword)
	tokens := loginTokens(t, srv, username, testPassword)

	// access token 与 refresh token 签名密钥不同，且带 tokenType 校验 → 401
	w := doJSON(t, srv, http.MethodPost, "/api/user/refresh", map[string]string{
		"refreshToken": tokens.AccessToken,
	}, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("access token 当 refresh 使用期望 HTTP 401，实际 %d", w.Code)
	}
}

// ==================== 补充：刷新后的新 access token 端到端可用 ====================

func TestRotatedAccessTokenWorks(t *testing.T) {
	srv := newTestServer()
	username := uniqueUsername("rotnew")
	registerUser(t, srv, username, testPassword)
	old := loginTokens(t, srv, username, testPassword)

	// 刷新拿到新 Token 对
	w := doJSON(t, srv, http.MethodPost, "/api/user/refresh", map[string]string{
		"refreshToken": old.RefreshToken,
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("刷新失败 HTTP=%d body=%s", w.Code, w.Body.String())
	}
	rotated := decodeTokens(t, decodeResult(t, w).Data)

	// 新 access token 可访问受保护接口
	w = doJSON(t, srv, http.MethodGet, "/api/user/info", nil, map[string]string{
		"Authorization": "Bearer " + rotated.AccessToken,
	})
	if w.Code != http.StatusOK {
		t.Errorf("刷新后的新 access token 期望 HTTP 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	var info struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(decodeResult(t, w).Data, &info); err != nil {
		t.Fatalf("解析用户信息失败: %v", err)
	}
	if info.Username != username {
		t.Errorf("期望 username=%s，实际 %s", username, info.Username)
	}
}

// ==================== 补充：并发刷新原子性（同一 refresh 只允许一个成功） ====================

func TestConcurrentRefreshOnlyOneWins(t *testing.T) {
	srv := newTestServer()
	username := uniqueUsername("race")
	registerUser(t, srv, username, testPassword)
	tokens := loginTokens(t, srv, username, testPassword)

	const n = 5
	results := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w := doJSON(t, srv, http.MethodPost, "/api/user/refresh", map[string]string{
				"refreshToken": tokens.RefreshToken,
			}, nil)
			results[idx] = w.Code
		}(i)
	}
	wg.Wait()

	okCount, failCount := 0, 0
	for _, c := range results {
		switch c {
		case http.StatusOK:
			okCount++
		case http.StatusUnauthorized:
			failCount++
		}
	}
	t.Logf("并发 %d 个刷新请求 => 成功 %d 个，401 %d 个", n, okCount, failCount)
	if okCount != 1 {
		t.Errorf("rotation 原子性失效：期望恰好 1 个成功，实际 %d 个", okCount)
	}
	if failCount != n-1 {
		t.Errorf("rotation 原子性失效：期望 %d 个被拒绝，实际 %d 个", n-1, failCount)
	}
}
