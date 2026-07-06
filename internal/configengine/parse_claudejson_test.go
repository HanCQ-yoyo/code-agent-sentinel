package configengine

import "testing"

func TestParseClaudeJSONProjectMCP(t *testing.T) {
	f := newFixture(t)
	f.writeClaudeJSON(`{
  "mcpServers": {"global-mcp": {"command": "g", "args": []}},
  "projects": {
    "/home/me/proj": {"mcpServers": {"proj-mcp": {"command": "p", "args": [], "env": {"TOKEN":"x"}}}}
  }
}`)

	got, err := parseClaudeJSONProjectMCP(f.cj, "/home/me/proj", ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1: %+v", len(got), got)
	}
	if got[0].Name != "proj-mcp" {
		t.Fatalf("name = %s, want proj-mcp", got[0].Name)
	}
	if got[0].Scope != ScopeProject {
		t.Fatalf("scope = %v, want project", got[0].Scope)
	}
	// 确认顶层 global-mcp 没被混入
	for _, a := range got {
		if a.Name == "global-mcp" {
			t.Fatal("global mcp leaked into project scope")
		}
	}
}

func TestParseClaudeJSONProjectMCPNoProject(t *testing.T) {
	f := newFixture(t)
	f.writeClaudeJSON(`{"projects": {}}`)
	got, err := parseClaudeJSONProjectMCP(f.cj, "/nope", ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

func TestParseClaudeJSONProjectMCPNoFile(t *testing.T) {
	// 文件不存在:不致错
	f := newFixture(t)
	got, err := parseClaudeJSONProjectMCP(f.cj, "/x", ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}
