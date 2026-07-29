import { describe, it, expect } from 'vitest'
import { treeToMatchMap, matchMapToTree, newLeaf } from './match-tree'

describe('match-tree smoke', () => {
  it('vitest 跑通', () => {
    expect(1 + 1).toBe(2)
  })

  it('newLeaf 产生空叶子', () => {
    expect(newLeaf()).toEqual({ type: 'leaf', field: '', op: '', value: '' })
  })
})
