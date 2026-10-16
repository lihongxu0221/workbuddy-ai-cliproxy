package models

import (
	"testing"
	"workbuddy-ai-cliproxy/internal/auth"
)

func TestParsePrefix(t *testing.T) {
	cases := []struct {
		input       string
		wantClean   string
		wantRealm   auth.Realm
		wantExplicit bool
	}{
		{
			input:        "cn/glm-5.2",
			wantClean:    "glm-5.2",
			wantRealm:    auth.RealmCN,
			wantExplicit: true,
		},
		{
			input:        "cn/hy3",
			wantClean:    "hy3",
			wantRealm:    auth.RealmCN,
			wantExplicit: true,
		},
		{
			input:        "cb/gpt-6-astra",
			wantClean:    "gpt-6-astra",
			wantRealm:    auth.RealmGlobal,
			wantExplicit: true,
		},
		{
			input:        "en/gpt-6.1-sol",
			wantClean:    "gpt-6.1-sol",
			wantRealm:    auth.RealmGlobal,
			wantExplicit: true,
		},
		{
			input:        "wb/claude-opus-5-5",
			wantClean:    "claude-opus-5-5",
			wantRealm:    auth.RealmGlobal,
			wantExplicit: true,
		},
		{
			input:        "workbuddy/cn/deepseek-v4-pro",
			wantClean:    "deepseek-v4-pro",
			wantRealm:    auth.RealmCN,
			wantExplicit: true,
		},
		{
			input:        "codebuddy/cb/gemini-3.8-flash",
			wantClean:    "gemini-3.8-flash",
			wantRealm:    auth.RealmGlobal,
			wantExplicit: true,
		},
		{
			// 无显式前缀，通过注册表先验推断为 CN
			input:        "hy3-preview",
			wantClean:    "hy3-preview",
			wantRealm:    auth.RealmCN,
			wantExplicit: false,
		},
		{
			// 无显式前缀，通过注册表先验推断为 Global
			input:        "gpt-6-luna",
			wantClean:    "gpt-6-luna",
			wantRealm:    auth.RealmGlobal,
			wantExplicit: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			res := ParsePrefix(tc.input)
			if res.CleanModel != tc.wantClean {
				t.Errorf("CleanModel = %s, want %s", res.CleanModel, tc.wantClean)
			}
			if res.TargetRealm != tc.wantRealm {
				t.Errorf("TargetRealm = %s, want %s", res.TargetRealm, tc.wantRealm)
			}
			if res.HasExplicit != tc.wantExplicit {
				t.Errorf("HasExplicit = %v, want %v", res.HasExplicit, tc.wantExplicit)
			}
		})
	}
}
