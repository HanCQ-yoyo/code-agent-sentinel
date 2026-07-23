package ruleengine

import "testing"

func TestDeobfuscate_WrapperStrip(t *testing.T) {
	cands := Deobfuscate("sudo rm -rf /", []string{"wrapper_strip"})
	// sudo 被剥离 → "rm -rf /"
	found := false
	for _, c := range cands {
		if c.Method == "wrapper_strip" && c.Text == "rm -rf /" {
			found = true
		}
	}
	if !found {
		t.Errorf("wrapper_strip should strip 'sudo', got %+v", cands)
	}
}

func TestDeobfuscate_WrapperStripEnvVar(t *testing.T) {
	// env VAR=1 rm -rf → rm -rf
	cands := Deobfuscate("env FOO=1 rm -rf /tmp", []string{"wrapper_strip"})
	found := false
	for _, c := range cands {
		if c.Method == "wrapper_strip" && c.Text == "rm -rf /tmp" {
			found = true
		}
	}
	if !found {
		t.Errorf("wrapper_strip should strip 'env FOO=1', got %+v", cands)
	}
}

func TestDeobfuscate_AnsiCDecode(t *testing.T) {
	// $'\x72\x6d' → rm(r=0x72, m=0x6d)
	cands := Deobfuscate("$'\\x72\\x6d' -rf /", []string{"ansi_c_decode"})
	found := false
	for _, c := range cands {
		if c.Method == "ansi_c_decode" && c.Text == "rm -rf /" {
			found = true
		}
	}
	if !found {
		t.Errorf("ansi_c_decode should decode $'\\x72\\x6d' to 'rm', got %+v", cands)
	}
}
