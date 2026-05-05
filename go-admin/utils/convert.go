package utils

import "fmt"

// ToString 任意类型转字符串
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
