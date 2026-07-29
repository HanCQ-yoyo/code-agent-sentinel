import { describe, it, expect } from 'vitest'
import {
  type MatchTreeNode,
  newLeaf,
  treeToMatchMap,
  matchMapToTree,
  wrapAs,
  unwrapTo,
  deleteNode,
  moveChild,
  isUnsupported,
} from './match-tree'

describe('treeToMatchMap', () => {
  it('叶子(标量 value)→ {field,op,value}', () => {
    const leaf: MatchTreeNode = { type: 'leaf', field: 'allow', op: 'contains', value: 'Bash(*)' }
    expect(treeToMatchMap(leaf)).toEqual({ field: 'allow', op: 'contains', value: 'Bash(*)' })
  })

  it('within 数组 value → {field,op,value:[]}', () => {
    const leaf: MatchTreeNode = { type: 'leaf', field: 'command', op: 'within', value: ['npx', 'docker'] }
    expect(treeToMatchMap(leaf)).toEqual({ field: 'command', op: 'within', value: ['npx', 'docker'] })
  })

  it('exists op(无 value)→ {field,op} 不含 value', () => {
    const leaf: MatchTreeNode = { type: 'leaf', field: 'env', op: 'exists', value: '' }
    expect(treeToMatchMap(leaf)).toEqual({ field: 'env', op: 'exists' })
  })

  it('空叶子 → {}', () => {
    expect(treeToMatchMap(newLeaf())).toEqual({})
  })

  it('and → {and:[...]}', () => {
    const node: MatchTreeNode = {
      type: 'and',
      children: [
        { type: 'leaf', field: 'raw', op: 'contains', value: 'skip' },
        { type: 'leaf', field: 'raw', op: 'regex_match', value: 'WebFetch' },
      ],
    }
    expect(treeToMatchMap(node)).toEqual({
      and: [
        { field: 'raw', op: 'contains', value: 'skip' },
        { field: 'raw', op: 'regex_match', value: 'WebFetch' },
      ],
    })
  })

  it('not → {not: map}(单个子,非数组)', () => {
    const node: MatchTreeNode = {
      type: 'not',
      child: { type: 'leaf', field: 'args', op: 'regex_match', value: '@sha256' },
    }
    expect(treeToMatchMap(node)).toEqual({
      not: { field: 'args', op: 'regex_match', value: '@sha256' },
    })
  })
})

describe('matchMapToTree', () => {
  it('叶子 map → leaf 节点', () => {
    expect(matchMapToTree({ field: 'allow', op: 'contains', value: 'Bash(*)' })).toEqual({
      type: 'leaf', field: 'allow', op: 'contains', value: 'Bash(*)',
    })
  })

  it('within 数组 value → leaf(value:[])', () => {
    expect(matchMapToTree({ field: 'command', op: 'within', value: ['npx'] })).toEqual({
      type: 'leaf', field: 'command', op: 'within', value: ['npx'],
    })
  })

  it('exists(无 value)→ leaf(value:"")', () => {
    expect(matchMapToTree({ field: 'env', op: 'exists' })).toEqual({
      type: 'leaf', field: 'env', op: 'exists', value: '',
    })
  })

  it('and/or → 分组节点', () => {
    const map = { or: [{ field: 'allow', op: 'contains', value: 'Edit(**)' }] }
    expect(matchMapToTree(map)).toEqual({
      type: 'or', children: [{ type: 'leaf', field: 'allow', op: 'contains', value: 'Edit(**)' }],
    })
  })

  it('not → not 节点(child 单 map)', () => {
    const map = { not: { field: 'args', op: 'regex_match', value: '@sha256' } }
    expect(matchMapToTree(map)).toEqual({
      type: 'not', child: { type: 'leaf', field: 'args', op: 'regex_match', value: '@sha256' },
    })
  })

  it('不支持形状(含特殊 op)→ null', () => {
    expect(matchMapToTree({ field: 'description', op: 'homoglyph_check' })).toBeNull()
  })

  it('不支持形状(布尔键混 field)→ null', () => {
    expect(matchMapToTree({ and: [], field: 'x' })).toBeNull()
  })

  it('空 map → null', () => {
    expect(matchMapToTree({})).toBeNull()
  })
})

describe('wrapAs', () => {
  it('叶子 → and(叶子作唯一子)', () => {
    const leaf = newLeaf()
    expect(wrapAs(leaf, 'and')).toEqual({ type: 'and', children: [leaf] })
  })

  it('叶子 → not(child=叶子)', () => {
    const leaf = newLeaf()
    expect(wrapAs(leaf, 'not')).toEqual({ type: 'not', child: leaf })
  })

  it('and → not(包裹整个 and)', () => {
    const andNode: MatchTreeNode = { type: 'and', children: [newLeaf()] }
    expect(wrapAs(andNode, 'not')).toEqual({ type: 'not', child: andNode })
  })
})

describe('unwrapTo', () => {
  it('not → and(子作首元素)', () => {
    const leaf = newLeaf()
    const notNode: MatchTreeNode = { type: 'not', child: leaf }
    expect(unwrapTo(notNode, 'and')).toEqual({ type: 'and', children: [leaf] })
  })

  it('and 单叶子子节点 → 该叶子', () => {
    const leaf = newLeaf()
    const andNode: MatchTreeNode = { type: 'and', children: [leaf] }
    expect(unwrapTo(andNode, 'leaf')).toEqual(leaf)
  })

  it('and 多子节点 → null(无法塌缩)', () => {
    const andNode: MatchTreeNode = { type: 'and', children: [newLeaf(), newLeaf()] }
    expect(unwrapTo(andNode, 'leaf')).toBeNull()
  })
})

describe('deleteNode', () => {
  it('删根 → 回退 newLeaf()', () => {
    expect(deleteNode(newLeaf(), [])).toEqual(newLeaf())
  })

  it('删 and 的第 0 子 → and 剩 1 子', () => {
    const root: MatchTreeNode = {
      type: 'and',
      children: [newLeaf(), { type: 'leaf', field: 'x', op: 'eq', value: 'y' }],
    }
    const result = deleteNode(root, [0])
    expect(result).toEqual({ type: 'and', children: [{ type: 'leaf', field: 'x', op: 'eq', value: 'y' }] })
  })
})

describe('moveChild', () => {
  it('and 子节点下移', () => {
    const root: MatchTreeNode = {
      type: 'and',
      children: [
        { type: 'leaf', field: 'a', op: 'eq', value: '1' },
        { type: 'leaf', field: 'b', op: 'eq', value: '2' },
      ],
    }
    const moved = moveChild(root, 0, 'down')
    // MatchTreeNode 把 and/or 合并为一个联合成员 { type: 'and' | 'or' },故 Extract 须匹配
    // 'and' | 'or'(匹配单 'and' 会得 never)。内层 leaf 单独成联合成员,Extract 'leaf' 正常。
    expect((moved as Extract<MatchTreeNode, { type: 'and' | 'or' }>).children[0].type === 'leaf' &&
      ((moved as Extract<MatchTreeNode, { type: 'and' | 'or' }>).children[0] as Extract<MatchTreeNode, { type: 'leaf' }>).field).toBe('b')
  })
})

describe('isUnsupported', () => {
  it('正常叶子 → false', () => {
    expect(isUnsupported({ field: 'allow', op: 'contains', value: 'x' })).toBe(false)
  })

  it('特殊 op → true', () => {
    expect(isUnsupported({ field: 'd', op: 'repeat_check' })).toBe(true)
  })
})
