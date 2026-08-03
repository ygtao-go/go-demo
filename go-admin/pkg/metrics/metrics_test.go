package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestMetricsMiddlewareRecordsHTTPMetrics 验证 HTTP 指标中间件：
//  1. 记录请求总数（method / path / status 维度）
//  2. 记录 HTTP 错误数（status >= 400）
//  3. /metrics 端点可正常返回 Prometheus 文本格式，且自身不计入请求总数
func TestMetricsMiddlewareRecordsHTTPMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	r.GET("/boom", func(c *gin.Context) { c.String(http.StatusInternalServerError, "boom") })
	r.GET(metricsPath, Handler())

	// 1. 正常请求：请求总数 +1
	before := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", "/ping", "200"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /ping 期望 HTTP 200，实际 %d", w.Code)
	}
	if after := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", "/ping", "200")); after != before+1 {
		t.Errorf("请求总数期望 %v，实际 %v", before+1, after)
	}

	// 2. 500 请求：请求总数 +1 且错误数 +1
	beforeErr := testutil.ToFloat64(HTTPErrorsTotal.WithLabelValues("GET", "/boom", "500"))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("GET /boom 期望 HTTP 500，实际 %d", w.Code)
	}
	if after := testutil.ToFloat64(HTTPErrorsTotal.WithLabelValues("GET", "/boom", "500")); after != beforeErr+1 {
		t.Errorf("HTTP 错误数期望 %v，实际 %v", beforeErr+1, after)
	}

	// 3. /metrics 端点可访问且返回 Prometheus 文本（自身不计入请求总数）
	beforeMetrics := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", metricsPath, "200"))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, metricsPath, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /metrics 期望 HTTP 200，实际 %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") && !strings.HasPrefix(ct, "application/openmetrics-text") {
		t.Errorf("Content-Type 期望 Prometheus 文本格式，实际 %q", ct)
	}
	body := w.Body.String()
	for _, name := range []string{"http_requests_total", "http_request_duration_seconds", "http_errors_total"} {
		if !strings.Contains(body, name) {
			t.Errorf("指标端点缺少指标 %s，body 片段: %s", name, body)
		}
	}
	if after := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", metricsPath, "200")); after != beforeMetrics {
		t.Errorf("/metrics 自身不应计入请求总数：before=%v after=%v", beforeMetrics, after)
	}
}

// TestRecordBusinessMetrics 验证业务指标上报函数。
func TestRecordBusinessMetrics(t *testing.T) {
	s0 := testutil.ToFloat64(RefreshSuccessTotal)
	f0 := testutil.ToFloat64(RefreshFailureTotal)
	a0 := testutil.ToFloat64(AICallsTotal)
	af0 := testutil.ToFloat64(AIFailuresTotal)

	RecordRefresh(true)
	RecordRefresh(false)
	RecordAICall(true)
	RecordAICall(false)

	if got := testutil.ToFloat64(RefreshSuccessTotal); got != s0+1 {
		t.Errorf("refresh 成功次数期望 %v，实际 %v", s0+1, got)
	}
	if got := testutil.ToFloat64(RefreshFailureTotal); got != f0+1 {
		t.Errorf("refresh 失败次数期望 %v，实际 %v", f0+1, got)
	}
	if got := testutil.ToFloat64(AICallsTotal); got != a0+2 {
		t.Errorf("AI 调用次数期望 %v，实际 %v", a0+2, got)
	}
	if got := testutil.ToFloat64(AIFailuresTotal); got != af0+1 {
		t.Errorf("AI 失败次数期望 %v，实际 %v", af0+1, got)
	}
}
