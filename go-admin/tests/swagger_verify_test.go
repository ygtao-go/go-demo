package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "go-admin/docs" // 注册 Swagger 文档实例（与 cmd/main.go 一致）
	"go-admin/router"

	"github.com/gin-gonic/gin"
)

// TestSwaggerEndpoints 运行时验证 /swagger/* 路由：UI 与 doc.json 均返回 200，
// 且 doc.json 中包含全部 15 个业务接口。
func TestSwaggerEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	router.InitRouter(r)

	for _, path := range []string{"/swagger/index.html", "/swagger/doc.json"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s 期望 HTTP 200，实际 %d", path, w.Code)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	r.ServeHTTP(w, req)

	var doc struct {
		Paths map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("解析 doc.json 失败: %v", err)
	}

	wantPaths := []string{
		"/user/register", "/user/login", "/user/refresh",
		"/user/info", "/user/logout", "/user/password",
		"/user", "/user/{id}", "/user/{id}/status",
		"/ai/generate", "/ai/generate/stream", "/ai/explain", "/ai/fix", "/ai/optimize",
		"/dashboard/statistics",
	}
	for _, p := range wantPaths {
		if _, ok := doc.Paths[p]; !ok {
			t.Errorf("doc.json 缺少接口 %s", p)
		}
	}
	t.Logf("Swagger 文档接口数量: %d", len(doc.Paths))
}
