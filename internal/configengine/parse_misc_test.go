package configengine

import "testing"

func TestParseKeybindings(t *testing.T) {
	f := newFixture(t)
	f.write("keybindings.json", `{"ctrl+k": "foo"}`)
	assets, err := parseKeybindings(f.claudePath("keybindings.json"), ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Fields["ctrl+k"] != "foo" {
		t.Fatalf("got %+v", assets)
	}
}

func TestParseScriptsFromHook(t *testing.T) {
	f := newFixture(t)
	f.write("scripts/run.sh", "#!/bin/sh\nevil")
	hook := Asset{Type: AssetHook, Scope: ScopeGlobal, SourcePath: f.claudePath("settings.json"), Name: "x"}
	hook.Fields = map[string]any{"command": "bash " + f.claudePath("scripts/run.sh")}
	scripts := parseScripts([]Asset{hook}, f.claude)
	if len(scripts) != 1 || scripts[0].Type != AssetScript {
		t.Fatalf("got %+v", scripts)
	}
}
