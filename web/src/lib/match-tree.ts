// match 树对象 ↔ RuleDTO.match map 双向转换 + 节点变换。纯函数,零依赖。
//
// 数据契约(对齐 internal/security/ruleengine/schema.go + validate.go):
//   叶子 = {field, op, value?};布尔 = {and|or: [map...]} / {not: map}
//   op 11 个分 4 类:无 value(exists/not_exists)/ 数组(within/not_within)/
//   标量(eq/not_equals/contains/not_contains)/ 正则(regex_match/not_regex_match/key_matches)
//   field 自由字符串。特殊 op(repeat_check/homoglyph_check)本编辑器不支持。

export type MatchTreeNode =
  | { type: 'leaf'; field: string; op: string; value: string | string[] }
  | { type: 'and' | 'or'; children: MatchTreeNode[] }
  | { type: 'not'; child: MatchTreeNode }

// 11 个用户 op(不含特殊 op)。
const USER_OPS = new Set([
  'exists', 'not_exists', 'within', 'not_within',
  'eq', 'not_equals', 'contains', 'not_contains',
  'regex_match', 'not_regex_match', 'key_matches',
])
const ARRAY_OPS = new Set(['within', 'not_within'])
const NO_VALUE_OPS = new Set(['exists', 'not_exists'])
const SPECIAL_OPS = new Set(['repeat_check', 'homoglyph_check'])

export function newLeaf(): MatchTreeNode {
  return { type: 'leaf', field: '', op: '', value: '' }
}

// 树对象 → match map。
export function treeToMatchMap(node: MatchTreeNode): Record<string, unknown> {
  switch (node.type) {
    case 'leaf': {
      if (!node.field && !node.op) return {} // 空叶子 → 空 match
      const m: Record<string, unknown> = { field: node.field, op: node.op }
      if (NO_VALUE_OPS.has(node.op)) return m // exists/not_exists 不写 value
      m.value = node.value
      return m
    }
    case 'and':
    case 'or':
      return { [node.type]: node.children.map(treeToMatchMap) }
    case 'not':
      return { not: treeToMatchMap(node.child) }
  }
}

// match map → 树对象。不支持形状 → null。
export function matchMapToTree(map: Record<string, unknown>): MatchTreeNode | null {
  if (!map || typeof map !== 'object' || Object.keys(map).length === 0) return null
  if (isUnsupported(map)) return null

  const boolKeys = ['and', 'or', 'not'].filter((k) => k in map)
  if (boolKeys.length > 1) return null

  if (boolKeys.length === 1) {
    const key = boolKeys[0]
    if ('field' in map || 'op' in map) return null // 布尔键混叶子字段 → 不支持
    if (key === 'not') {
      const child = matchMapToTree(map.not as Record<string, unknown>)
      return child ? { type: 'not', child } : null
    }
    const arr = map[key]
    if (!Array.isArray(arr) || arr.length === 0) return null
    const children = arr.map((e) => matchMapToTree(e as Record<string, unknown>))
    if (children.some((c) => c === null)) return null
    return { type: key as 'and' | 'or', children: children as MatchTreeNode[] }
  }

  // 叶子
  const field = map.field
  const op = map.op
  if (typeof field !== 'string' || typeof op !== 'string') return null
  if (!USER_OPS.has(op)) return null // 含特殊 op / 未知 op
  let value: string | string[]
  if (NO_VALUE_OPS.has(op)) {
    value = ''
  } else if (ARRAY_OPS.has(op)) {
    if (!Array.isArray(map.value)) return null
    value = (map.value as unknown[]).map(String)
  } else {
    if (typeof map.value !== 'string') return null
    value = map.value
  }
  return { type: 'leaf', field, op, value }
}

// 判断 match map 是否含本编辑器不支持的写法(混键 / 含特殊 op / 未知 op / 异形)。
export function isUnsupported(map: Record<string, unknown>): boolean {
  if (!map || typeof map !== 'object') return true
  const keys = Object.keys(map)
  if (keys.length === 0) return false // 空 map 由 matchMapToTree 返回 null,不算 unsupported
  const boolKeys = keys.filter((k) => k === 'and' || k === 'or' || k === 'not')
  const leafKeys = keys.filter((k) => k === 'field' || k === 'op' || k === 'value')
  // 布尔节点:仅含一个布尔键(子元素递归判断)
  if (boolKeys.length === 1 && leafKeys.length === 0) {
    const key = boolKeys[0]
    if (key === 'not') {
      const child = map.not
      return typeof child !== 'object' || child === null || isUnsupported(child as Record<string, unknown>)
    }
    const arr = map[key]
    if (!Array.isArray(arr)) return true
    return arr.some((e) => typeof e !== 'object' || e === null || isUnsupported(e as Record<string, unknown>))
  }
  // 叶子节点:须有 field+op,op 是用户 op
  if (boolKeys.length === 0) {
    const op = map.op
    if (typeof op !== 'string') return true
    if (SPECIAL_OPS.has(op)) return true
    if (!USER_OPS.has(op)) return true
    return false
  }
  return true // 混布尔+叶子键
}

// 叶子 → and/or/not(叶子作唯一子);and/or → not 包裹。
export function wrapAs(node: MatchTreeNode, kind: 'and' | 'or' | 'not'): MatchTreeNode {
  if (kind === 'not') return { type: 'not', child: node }
  return { type: kind, children: [node] }
}

// not → and/or(子作首元素);and/or 单叶子子节点 → 该叶子。无法变换 → null。
export function unwrapTo(node: MatchTreeNode, kind: 'and' | 'or' | 'leaf'): MatchTreeNode | null {
  if (node.type === 'not') {
    if (kind === 'leaf') return node.child
    return { type: kind, children: [node.child] }
  }
  if (node.type === 'and' || node.type === 'or') {
    if (kind === 'leaf') {
      if (node.children.length === 1 && node.children[0].type === 'leaf') return node.children[0]
      return null
    }
    return null // and↔or 直接改 type 即可,但此处保留语义:不通过 unwrap
  }
  return null // 叶子无法 unwrap
}

// 按 path(子索引数组)删节点。删根(path=[])→ 回退 newLeaf()。
export function deleteNode(root: MatchTreeNode, path: number[]): MatchTreeNode {
  if (path.length === 0) return newLeaf()
  const [idx, ...rest] = path
  if (root.type === 'and' || root.type === 'or') {
    if (rest.length === 0) {
      const children = root.children.filter((_, i) => i !== idx)
      if (children.length === 0) return newLeaf() // 空分组回退
      return { type: root.type, children }
    }
    const child = root.children[idx]
    return { type: root.type, children: root.children.map((c, i) => (i === idx ? deleteNode(child, rest) : c)) }
  }
  if (root.type === 'not') {
    if (rest.length === 0) return newLeaf()
    return { type: 'not', child: deleteNode(root.child, rest) }
  }
  return root
}

// and/or 子节点上下移。越界/非分组 → 原样返回。
export function moveChild(node: MatchTreeNode, index: number, dir: 'up' | 'down'): MatchTreeNode {
  if (node.type !== 'and' && node.type !== 'or') return node
  const children = [...node.children]
  const target = dir === 'up' ? index - 1 : index + 1
  if (target < 0 || target >= children.length) return node
  ;[children[index], children[target]] = [children[target], children[index]]
  return { type: node.type, children }
}
