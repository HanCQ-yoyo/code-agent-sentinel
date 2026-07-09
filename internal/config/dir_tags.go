package config

// DirTags 是「目录标签」覆盖映射:key = 相对 .claude 根的目录/文件路径
// (如 "sessions"、"security"、"plugins/data"),value = 标签名。
//
// 仅存用户显式覆盖;默认标签由 DefaultDirTags() 提供。生效标签 = 覆盖优先,
// 否则取默认。继承:某路径未直接命中时,取最近祖先目录的标签(最长前缀匹配),
// 无命中则视为无标签("untagged")。详见 ResolveDirTag。
type DirTags = map[string]string

// 标签常量(避免散落字符串)。
const (
	TagRuntime = "runtime" // 运行时/缓存/状态文件(非用户管理的配置)
	TagConfig  = "config"  // 用户管理的配置资产
)

// DefaultDirTags 返回内置默认标签:key = 相对 .claude 根的路径,value = 标签。
//
// 依据 Claude Code 实际目录布局:
//   - config:settings.json / settings.local.json / keybindings.json / memory /
//     skills / commands / agents / plugins(插件资产可被发现)/ .mcp.json(项目根)。
//   - runtime:sessions / session-env / shell-snapshots / file-history / telemetry /
//     history.jsonl / backups / transcripts / ide / projects / tasks / teams /
//     security(警告状态+日志)/ .last-cleanup / .credentials.json /
//     mcp-needs-auth-cache.json / plugins/data / plugins/cache/temp_git_*。
//
// 这些是「合理默认」,用户可经 /api/dir-tags 覆盖任意路径。
func DefaultDirTags() DirTags {
	return DirTags{
		// config(用户管理的配置资产)
		"settings.json":          TagConfig,
		"settings.local.json":    TagConfig,
		"keybindings.json":       TagConfig,
		"memory":                 TagConfig,
		"skills":                 TagConfig,
		"commands":               TagConfig,
		"agents":                 TagConfig,
		"plugins":                TagConfig, // 顶层 plugins;data/cache 子目录另行标 runtime
		".mcp.json":              TagConfig,
		// runtime(运行时/缓存/状态)
		"sessions":                     TagRuntime,
		"session-env":                  TagRuntime,
		"shell-snapshots":              TagRuntime,
		"file-history":                 TagRuntime,
		"telemetry":                    TagRuntime,
		"history.jsonl":                TagRuntime,
		"backups":                      TagRuntime,
		"transcripts":                  TagRuntime,
		"ide":                          TagRuntime,
		"projects":                     TagRuntime,
		"tasks":                        TagRuntime,
		"teams":                        TagRuntime,
		"security":                     TagRuntime,
		".last-cleanup":                TagRuntime,
		".credentials.json":            TagRuntime,
		"mcp-needs-auth-cache.json":    TagRuntime,
		"plugins/data":                 TagRuntime,
		"plugins/cache/temp_git":       TagRuntime, // temp_git_* 下载缓存
	}
}

// ResolveDirTag 计算单个相对路径的生效标签:
//  1. 覆盖优先:若 overrides 含该路径(或其祖先的最长前缀)用覆盖;
//  2. 否则默认:DefaultDirTags 同理最长前缀;
//  3. 都不命中返回 ""(untagged)。
//
// 「最长前缀匹配」使目录标签继承给子项:标了 "sessions" 为 runtime,
// 其下 sessions/3000363.json 也算 runtime。前缀按路径段匹配(用 segHasPrefix),
// 防 "plugins" 误匹配 "plugins-data"。
func ResolveDirTag(rel string, overrides DirTags) string {
	if t, ok := longestPrefix(rel, overrides); ok {
		return t
	}
	if t, ok := longestPrefix(rel, DefaultDirTags()); ok {
		return t
	}
	return ""
}

// longestPrefix 在 m 里找 rel 的最长前缀路径(按路径段),返回其标签。
func longestPrefix(rel string, m DirTags) (string, bool) {
	best := ""
	bestTag := ""
	found := false
	for k, t := range m {
		if segHasPrefix(rel, k) {
			// 取最长(最深)前缀:更具体的覆盖更浅的。
			if !found || len(k) > len(best) {
				best = k
				bestTag = t
				found = true
			}
		}
	}
	return bestTag, found
}

// segHasPrefix 判断 path 是否以 prefix 开头(按完整路径段,非字符串前缀)。
// "sessions/x" 对 "sessions" → true;"sessions-foo" 对 "sessions" → false;
// path == prefix → true。
func segHasPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	if prefix == "" {
		return true
	}
	// path 必须形如 prefix + "/" + ...
	if len(path) <= len(prefix) {
		return false
	}
	if path[:len(prefix)] != prefix {
		return false
	}
	return path[len(prefix)] == '/'
}
