package handler

import (
	"go-admin/internal/dto"
	"go-admin/internal/service"
	"go-admin/pkg/response"

	"github.com/gin-gonic/gin"
)

// ==================== Dashboard 模块 ====================

// GetDashboardStatistics 获取 Dashboard 统计数据
// @Summary 获取 Dashboard 统计数据
// @Description 返回用户总数、AI 调用/失败次数、HTTP 请求/错误次数；用户数来自 MySQL users 表，业务计数来自 Redis dashboard:* 计数器
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.DashboardResult "统计成功"
// @Failure 401 {object} response.Result "未登录或 token 无效"
// @Failure 500 {object} response.Result "统计数据获取失败"
// @Router /dashboard/statistics [get]
func GetDashboardStatistics(c *gin.Context) {
	stats, err := service.GetDashboardStatistics()
	if err != nil {
		response.Fail(c, 500, "统计数据获取失败")
		return
	}

	// 组装标准响应信封（dto.DashboardResult 仅用于 Swagger 文档化；
	// 运行时 code/msg 由 pkg/response.Success 统一生成，保证与全项目一致）
	result := dto.DashboardResult{
		Code: 0,
		Msg:  "success",
		Data: stats,
	}
	response.Success(c, result.Data)
}
