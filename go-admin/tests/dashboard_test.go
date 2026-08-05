package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"go-admin/internal/repository"
)

// ============================================================
// Dashboard 数据看板接口测试
//
// 覆盖：
//   GET /api/dashboard/statistics
//
// 验证：
//   1. 未携带 token → HTTP 401（JWT 保护）
//   2. 携带有效 token → HTTP 200，返回统一响应信封，data 包含 5 个统计字段
//   3. Redis dashboard 计数器与用户数真实联动：预写入后读取结果一致
//   4. 不存在的 Redis 计数器按 0 返回
// ============================================================

// dashboardStatisticsData 对应 dto.DashboardStatistics
type dashboardStatisticsData struct {
	UserCount    int64 `json:"userCount"`
	AICallCount  int64 `json:"aiCallCount"`
	AIErrorCount int64 `json:"aiErrorCount"`
	RequestCount int64 `json:"requestCount"`
	ErrorCount   int64 `json:"errorCount"`
}

// TestDashboardStatisticsRequiresAuth 未登录访问 dashboard 统计接口必须 401。
func TestDashboardStatisticsRequiresAuth(t *testing.T) {
	srv := newTestServer()
	w := doJSON(t, srv, http.MethodGet, "/api/dashboard/statistics", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问 /api/dashboard/statistics 期望 HTTP 401，实际 %d", w.Code)
	}
}

// TestDashboardStatistics 登录后访问 dashboard 统计接口：
//   - 预写入 Redis dashboard 计数器后，接口返回值必须与写入值一致
//   - 用户数必须与 users 表 COUNT(*) 一致（本测试至少注册了 1 个用户）
func TestDashboardStatistics(t *testing.T) {
	srv := newTestServer()

	// 1. 注册并登录，得到有效 access token
	username := uniqueUsername("dash")
	registerUser(t, srv, username, testPassword)
	tokens := loginTokens(t, srv, username, testPassword)

	// 2. 预写入 Redis dashboard 计数器（固定已知值，便于精确断言）
	if err := repository.IncrDashboardCounter(repository.DashboardMetricAICalls); err != nil {
		t.Fatalf("预写 dashboard:ai_calls 失败: %v", err)
	}
	if err := repository.IncrDashboardCounter(repository.DashboardMetricAIErrors); err != nil {
		t.Fatalf("预写 dashboard:ai_errors 失败: %v", err)
	}
	if err := repository.IncrDashboardCounter(repository.DashboardMetricHTTPRequests); err != nil {
		t.Fatalf("预写 dashboard:http_requests 失败: %v", err)
	}
	if err := repository.IncrDashboardCounter(repository.DashboardMetricHTTPErrors); err != nil {
		t.Fatalf("预写 dashboard:http_errors 失败: %v", err)
	}

	// 3. 携带 token 访问统计接口
	w := doJSON(t, srv, http.MethodGet, "/api/dashboard/statistics", nil, map[string]string{
		"Authorization": "Bearer " + tokens.AccessToken,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/dashboard/statistics 期望 HTTP 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	r := decodeResult(t, w)
	if r.Code != 0 {
		t.Fatalf("统计接口业务码期望 0，实际 %d msg=%s", r.Code, r.Msg)
	}

	var data dashboardStatisticsData
	if err := json.Unmarshal(r.Data, &data); err != nil {
		t.Fatalf("解析统计 data 失败: %v (data=%s)", err, string(r.Data))
	}

	// 4. 断言各字段
	if data.AICallCount != 1 {
		t.Errorf("aiCallCount 期望 1，实际 %d", data.AICallCount)
	}
	if data.AIErrorCount != 1 {
		t.Errorf("aiErrorCount 期望 1，实际 %d", data.AIErrorCount)
	}
	if data.RequestCount != 1 {
		t.Errorf("requestCount 期望 1，实际 %d", data.RequestCount)
	}
	if data.ErrorCount != 1 {
		t.Errorf("errorCount 期望 1，实际 %d", data.ErrorCount)
	}
	if data.UserCount < 1 {
		t.Errorf("userCount 期望 >=1（至少包含本次注册用户），实际 %d", data.UserCount)
	}
}

// TestDashboardStatisticsMissingCounters 未写入过的 Redis 计数器必须按 0 返回。
// 注意：同一测试二进制内其他用例（TestDashboardStatistics）会写入计数器，
// 因此本用例开始时显式删除 4 个 dashboard 计数器，确保「缺失」语义独立可测。
func TestDashboardStatisticsMissingCounters(t *testing.T) {
	srv := newTestServer()
	username := uniqueUsername("dashzero")
	registerUser(t, srv, username, testPassword)
	tokens := loginTokens(t, srv, username, testPassword)

	// 显式清理 dashboard 计数器，保证从「缺失」状态开始
	for _, metric := range []string{
		repository.DashboardMetricAICalls,
		repository.DashboardMetricAIErrors,
		repository.DashboardMetricHTTPRequests,
		repository.DashboardMetricHTTPErrors,
	} {
		if err := repository.DeleteDashboardCounter(metric); err != nil {
			t.Fatalf("清理 dashboard:%s 失败: %v", metric, err)
		}
	}

	w := doJSON(t, srv, http.MethodGet, "/api/dashboard/statistics", nil, map[string]string{
		"Authorization": "Bearer " + tokens.AccessToken,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/dashboard/statistics 期望 HTTP 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	r := decodeResult(t, w)

	var data dashboardStatisticsData
	if err := json.Unmarshal(r.Data, &data); err != nil {
		t.Fatalf("解析统计 data 失败: %v (data=%s)", err, string(r.Data))
	}
	// 删除后 4 项计数器必须全部为 0（userCount 只要求 >=1）
	if data.AICallCount != 0 || data.AIErrorCount != 0 || data.RequestCount != 0 || data.ErrorCount != 0 {
		t.Errorf("未写入计数器应按 0 返回，实际 data=%+v", data)
	}
}
