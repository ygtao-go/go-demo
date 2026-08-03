package utils

import "fmt"

// Deprecated: ToString 已无任何内部调用方，保留以备外部兼容；新代码请勿使用。
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
