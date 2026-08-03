package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Result struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Result{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

// Success200 兼容 AI 模块旧接口：业务码固定为 200。
// web 前端对 /api/ai/* 接口依赖 data.code === 200 判断成功，不能改为 0。
func Success200(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Result{
		Code: 200,
		Msg:  "success",
		Data: data,
	})
}

func Fail(c *gin.Context, code int, msg string) {
	c.JSON(code, Result{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}
