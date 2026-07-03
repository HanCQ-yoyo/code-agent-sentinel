package api

import (
	"testing"

	"code-agent-sentinel/internal/config"
)

func TestValidateLoopbackOK(t *testing.T) {
	if err := ValidateBindPolicy(&config.Config{Bind: "127.0.0.1"}, false); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNonLoopbackRequiresAllowlist(t *testing.T) {
	err := ValidateBindPolicy(&config.Config{Bind: "0.0.0.0"}, false)
	if err == nil {
		t.Fatal("非 loopback 无白名单应报错")
	}
}

func TestValidateNonLoopbackWithAllowlist(t *testing.T) {
	if err := ValidateBindPolicy(&config.Config{Bind: "0.0.0.0", AllowedCIDRs: []string{"10.0.0.0/8"}}, false); err != nil {
		t.Fatal(err)
	}
}

func TestValidateOverrideRisky(t *testing.T) {
	if err := ValidateBindPolicy(&config.Config{Bind: "0.0.0.0"}, true); err != nil {
		t.Fatal("override 应放行")
	}
}

// TestValidateRejectsAllMalformedCIDRs 验证 I-SEC-2:
// allowed_cidrs 非空但全部无法解析时,ValidateBindPolicy 应在启动时拒绝
// (fail-closed),而非静默放行所有流量。旧实现只检查 len==0,全部畸形时
// parseCIDRs 返回空 → clientIPGuard 早期 c.Next() → 允许所有访问。
func TestValidateRejectsAllMalformedCIDRs(t *testing.T) {
	err := ValidateBindPolicy(&config.Config{Bind: "0.0.0.0", AllowedCIDRs: []string{"not-a-cid", "also-bad!"}}, false)
	if err == nil {
		t.Fatal("全部 CIDR 畸形应在启动时拒绝(fail-closed),非静默放行")
	}
}

// TestValidateAcceptsPartialMalformedCIDRs 验证至少一个 CIDR 可解析时放行:
// 不可因部分畸形而拒绝整个配置(可解析的仍构成有效白名单)。
func TestValidateAcceptsPartialMalformedCIDRs(t *testing.T) {
	if err := ValidateBindPolicy(&config.Config{Bind: "0.0.0.0", AllowedCIDRs: []string{"not-a-cid", "10.0.0.0/8"}}, false); err != nil {
		t.Fatalf("至少一个 CIDR 可解析应放行, got %v", err)
	}
}
