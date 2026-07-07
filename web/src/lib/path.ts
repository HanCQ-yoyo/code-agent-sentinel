// 截取路径到 .claude/ 之后(相对 ~/.claude/ 的展示路径)。
// 前端无 home 根,用 .claude/ 启发式;不含 .claude/ 则原样返回(如 managed 外部文件)。
// 阶段 B 引入 agent 概念后会改为相对真实 agent 根目录。
export function relativeClaudePath(abs: string): string {
  const marker = '.claude/'
  const i = abs.indexOf(marker)
  if (i === -1) return abs
  return abs.slice(i + marker.length) || '.'
}
