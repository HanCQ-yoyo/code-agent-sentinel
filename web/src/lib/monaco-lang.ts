// langByExt 按 sourcePath 的扩展名返回 Monaco 语言 ID。
// script 资产的 source_path 是脚本绝对路径,取 basename 后的扩展名映射。
// 未知/无扩展名 → plaintext(Monaco 内置,主线程 Monarch 分词,无需 worker)。
export function langByExt(sourcePath: string): string {
  const base = sourcePath.split('/').pop() ?? sourcePath
  const i = base.lastIndexOf('.')
  if (i <= 0) return 'plaintext' // 无扩展名或隐藏文件(如 .bashrc)→ plaintext
  const ext = base.slice(i + 1).toLowerCase()
  const map: Record<string, string> = {
    sh: 'shell',
    bash: 'shell',
    zsh: 'shell',
    py: 'python',
    js: 'javascript',
    mjs: 'javascript',
    cjs: 'javascript',
    ts: 'typescript',
    go: 'go',
    json: 'json',
    md: 'markdown',
    yaml: 'yaml',
    yml: 'yaml',
  }
  return map[ext] ?? 'plaintext'
}
