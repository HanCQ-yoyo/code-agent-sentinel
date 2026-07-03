package configengine

import "testing"

func TestDetectDuplicates(t *testing.T) {
	assets := []Asset{
		{ID: "1", Type: AssetMCPServer, Scope: ScopeGlobal, Name: "gmail", SourcePath: "/a"},
		{ID: "2", Type: AssetMCPServer, Scope: ScopeProject, Name: "gmail", SourcePath: "/b"},
		{ID: "3", Type: AssetSkill, Scope: ScopeGlobal, Name: "s1", SourcePath: "/c"},
	}
	dups := detectDuplicates(assets)
	if len(dups) != 1 || dups[0].Name != "gmail" {
		t.Fatalf("want 1 dup 'gmail', got %+v", dups)
	}
}
