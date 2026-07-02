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
