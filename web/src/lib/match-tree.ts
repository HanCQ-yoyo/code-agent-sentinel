// 占位:Task 2 替换为完整实现。
export type MatchTreeNode =
  | { type: 'leaf'; field: string; op: string; value: string | string[] }
  | { type: 'and' | 'or'; children: MatchTreeNode[] }
  | { type: 'not'; child: MatchTreeNode }

export function newLeaf(): MatchTreeNode {
  return { type: 'leaf', field: '', op: '', value: '' }
}

export function treeToMatchMap(_node: MatchTreeNode): Record<string, unknown> {
  return {}
}

export function matchMapToTree(_map: Record<string, unknown>): MatchTreeNode | null {
  return newLeaf()
}
