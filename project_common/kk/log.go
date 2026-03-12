package kk

import (
	"encoding/json"
	"go_project/ms_project/project_common/tms"
	"time"
)

type FieldMap map[string]any

var serviceName = "unknown" // 当前服务名

// 设置当前服务名（"project", "user"）
func SetServiceName(name string) {
	serviceName = name
}

type KafkaLog struct {
	Type        string   `json:"type"`         // 日志级别：info / error / warn
	Action      string   `json:"action"`       // 操作类型：create / update / delete / query / login
	ServiceName string   `json:"service_name"` // 服务名：project / user / api
	Time        string   `json:"time"`         // 时间戳
	Msg         string   `json:"msg"`          // 日志消息
	Field       FieldMap `json:"field"`        // 业务字段
	FuncName    string   `json:"func_name"`    // 函数名
}

func Error(err error, funcName string, fieldMap FieldMap) []byte {
	kl := KafkaLog{
		Type:        "error",
		Action:      fieldMap.getAction(),
		ServiceName: serviceName,
		Time:        tms.Format(time.Now()),
		Msg:         err.Error(),
		Field:       fieldMap,
		FuncName:    funcName,
	}
	bytes, _ := json.Marshal(kl)
	return bytes
}

func Info(msg string, funcName string, fieldMap FieldMap) []byte {
	kl := KafkaLog{
		Type:        "info",
		Action:      msg,
		ServiceName: serviceName,
		Time:        tms.Format(time.Now()),
		Msg:         msg,
		Field:       fieldMap,
		FuncName:    funcName,
	}
	bytes, _ := json.Marshal(kl)
	return bytes
}

func Warn(msg string, funcName string, fieldMap FieldMap) []byte {
	kl := KafkaLog{
		Type:        "warn",
		Action:      msg,
		ServiceName: serviceName,
		Time:        tms.Format(time.Now()),
		Msg:         msg,
		Field:       fieldMap,
		FuncName:    funcName,
	}
	bytes, _ := json.Marshal(kl)
	return bytes
}

// 从 FieldMap 中提取 action
func (f FieldMap) getAction() string {
	if f == nil {
		return "unknown"
	}
	if v, ok := f["action"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "unknown"
}
