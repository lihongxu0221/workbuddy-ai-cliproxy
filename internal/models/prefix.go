package models

import (
	"strings"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
)

// ParsedModel 经过前缀解构的模型定位结果
type ParsedModel struct {
	OriginalName string
	CleanModel   string
	TargetRealm  auth.Realm
	HasExplicit  bool
}

// ParsePrefix 解析客户端传入的模型名称并确定目标区域
func ParsePrefix(rawModel string) ParsedModel {
	cfg := config.Get()
	cnPrefix := cfg.CNPrefix
	if cnPrefix == "" {
		cnPrefix = "cn/"
	}
	globalPrefix := cfg.GlobalPrefix
	if globalPrefix == "" {
		globalPrefix = "cb/"
	}

	res := ParsedModel{
		OriginalName: rawModel,
		CleanModel:   rawModel,
		TargetRealm:  auth.RealmGlobal, // 默认偏向 Global
		HasExplicit:  false,
	}

	if strings.HasPrefix(rawModel, cnPrefix) {
		res.CleanModel = strings.TrimPrefix(rawModel, cnPrefix)
		res.TargetRealm = auth.RealmCN
		res.HasExplicit = true
		return res
	}

	if strings.HasPrefix(rawModel, globalPrefix) {
		res.CleanModel = strings.TrimPrefix(rawModel, globalPrefix)
		res.TargetRealm = auth.RealmGlobal
		res.HasExplicit = true
		return res
	}

	// 兼容通用别名 "en/"、"wb/"、"workbuddy/"、"codebuddy/"
	if strings.HasPrefix(rawModel, "en/") {
		res.CleanModel = strings.TrimPrefix(rawModel, "en/")
		res.TargetRealm = auth.RealmGlobal
		res.HasExplicit = true
		return res
	}

	if strings.HasPrefix(rawModel, "wb/") {
		res.CleanModel = strings.TrimPrefix(rawModel, "wb/")
		res.TargetRealm = auth.RealmGlobal
		res.HasExplicit = true
		return res
	}

	if strings.HasPrefix(rawModel, "workbuddy/") {
		res.CleanModel = strings.TrimPrefix(rawModel, "workbuddy/")
		res.HasExplicit = true
		// 判断去除前缀后是否还有子前缀
		return ParsePrefix(res.CleanModel)
	}

	if strings.HasPrefix(rawModel, "codebuddy/") {
		res.CleanModel = strings.TrimPrefix(rawModel, "codebuddy/")
		res.HasExplicit = true
		return ParsePrefix(res.CleanModel)
	}

	// 若未带显式前缀，通过注册表先验推断
	reg := GetRegistry()
	if info := reg.Get(res.CleanModel); info != nil {
		res.TargetRealm = info.Realm
	}

	return res
}
