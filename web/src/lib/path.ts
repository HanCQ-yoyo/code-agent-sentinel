// 截取路径到 agent 配置根(.claude/)之后的展示路径。
// 前端无 home 根,用 .claude/ 启发式;不含 .claude/ 则原样返回(如根外资产)。
// 阶段 B:相对「当前 agent 根」(Claude Code = ~/.claude),语义与旧实现一致,
// 注释更新去掉 TODO(agent 概念已落地,agent 根 = .claude/)。
export function relativeClaudePath(abs: string): string {
  const marker = '.claude/'
  const i = abs.indexOf(marker)
  if (i === -1) return abs
  return abs.slice(i + marker.length) || '.'
}
