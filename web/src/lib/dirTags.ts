// 目录标签(dir tag)前端逻辑:与后端 internal/config/dir_tags.go 保持一致。
//
// 标签语义:
//   - config: 用户管理的配置资产目录/文件
//   - runtime: 运行时/缓存/状态(非用户管理)
//
// 生效标签 = 覆盖优先(最长前缀匹配),否则默认,否则 ""(untagged)。
// 路径相对于「当前树根」(全局 .claude 或项目 .claude)。

export type DirTag = 'config' | 'runtime'
export type DirTagsMap = Record<string, DirTag>

export const TAG_CONFIG: DirTag = 'config'
export const TAG_RUNTIME: DirTag = 'runtime'

// segHasPrefix:按完整路径段判断 path 是否以 prefix 开头(非字符串前缀)。
// "sessions/x" 对 "sessions" → true;"sessions-foo" 对 "sessions" → false。
function segHasPrefix(path: string, prefix: string): boolean {
  if (path === prefix) return true
  if (prefix === '') return true
  if (path.length <= prefix.length) return false
  if (!path.startsWith(prefix)) return false
  return path[prefix.length] === '/'
}

// longestPrefix:在 m 里找 path 的最长前缀路径(按路径段),返回其标签。
function longestPrefix(path: string, m: DirTagsMap): DirTag | undefined {
  let best = ''
  let bestTag: DirTag | undefined
  let found = false
  for (const [k, t] of Object.entries(m)) {
    if (segHasPrefix(path, k)) {
      if (!found || k.length > best.length) {
        best = k
        bestTag = t
        found = true
      }
    }
  }
  return bestTag
}

// resolveDirTag 计算单个相对路径的生效标签:覆盖优先,否则默认,否则 undefined。
export function resolveDirTag(rel: string, defaults: DirTagsMap, overrides: DirTagsMap): DirTag | undefined {
  const ov = longestPrefix(rel, overrides)
  if (ov) return ov
  const def = longestPrefix(rel, defaults)
  if (def) return def
  return undefined
}
