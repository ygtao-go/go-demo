package api

import (
	"go-admin/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 注意：函数名首字母必须大写！
func GenerateCode(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	_ = c.ShouldBindJSON(&req)
	res, _ := utils.CallAI("生成代码：" + req.Prompt)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": res})
}

func ExplainCode(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	_ = c.ShouldBindJSON(&req)
	res, _ := utils.CallAI("解释这段代码：\n" + req.Code)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": res})
}

func FixCode(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	_ = c.ShouldBindJSON(&req)
	res, _ := utils.CallAI("修复这段代码的错误：\n" + req.Code)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": res})
}

func OptimizeCode(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	_ = c.ShouldBindJSON(&req)
	res, _ := utils.CallAI("优化这段代码，让它更简洁高效：\n" + req.Code)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": res})
}
