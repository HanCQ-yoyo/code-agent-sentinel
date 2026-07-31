import { Alert } from 'antd'
import { useTranslation } from 'react-i18next'
import type { MatchTreeNode } from '../lib/match-tree'
import { wrapAs, unwrapTo, deleteNode, moveChild, newLeaf } from '../lib/match-tree'
import { MatchNodeRow } from './MatchNodeRow'

export interface MatchTreeEditorProps {
  value: MatchTreeNode | null
  matchMap: Record<string, unknown>
  assetType: string
  readOnly: boolean
  onChange: (next: MatchTreeNode) => void
  onUnsupportedChange?: (map: Record<string, unknown>) => void
}

// 递归渲染整棵 match 树。value=null(不支持形状)→ 降级只读 JSON 块 + 警告。
// 根节点变换/删除回退:删根 → newLeaf()。
export function MatchTreeEditor({ value, matchMap, assetType, readOnly, onChange }: MatchTreeEditorProps) {
  const { t } = useTranslation()

  // 不支持形状:降级只读 JSON 块(保存时 RuleDrawer 用原 matchMap 回写,见 Task 7)。
  if (value === null) {
    return (
      <div>
        <Alert
          type="warning"
          showIcon
          message={t('ruleForm.unsupportedMatch')}
          description={t('ruleForm.unsupportedMatchHint')}
          style={{ marginBottom: 8 }}
        />
        <pre style={{ margin: 0, fontSize: 'var(--fs-sm)', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
          {JSON.stringify(matchMap, null, 2)}
        </pre>
      </div>
    )
  }

  // 根节点回调:path=[] 直接作用于 root。
  const rootChange = (next: MatchTreeNode) => onChange(next)
  const rootWrap = (kind: 'and' | 'or' | 'not') => onChange(wrapAs(value, kind))
  const rootUnwrap = (kind: 'and' | 'or' | 'leaf') => {
    const u = unwrapTo(value, kind)
    if (u) onChange(u)
  }
  const rootDelete = () => onChange(deleteNode(value, []))

  // and/or 子节点操作(作用于 root 直接子)。
  const addChild = (kind: 'leaf' | 'group') => {
    if (value.type !== 'and' && value.type !== 'or') return
    const child: MatchTreeNode = kind === 'leaf' ? newLeaf() : { type: 'and', children: [newLeaf()] }
    onChange({ type: value.type, children: [...value.children, child] })
  }
  const moveChildHandler = (index: number, dir: 'up' | 'down') => {
    if (value.type !== 'and' && value.type !== 'or') return
    onChange(moveChild(value, index, dir))
  }
  const childChange = (index: number, next: MatchTreeNode) => {
    if (value.type !== 'and' && value.type !== 'or') return
    onChange({ type: value.type, children: value.children.map((c, i) => (i === index ? next : c)) })
  }
  const childDelete = (index: number) => {
    if (value.type !== 'and' && value.type !== 'or') return
    const children = value.children.filter((_, i) => i !== index)
    if (children.length === 0) { onChange(newLeaf()); return }
    onChange({ type: value.type, children })
  }
  const childWrap = (index: number, kind: 'and' | 'or' | 'not') => {
    if (value.type !== 'and' && value.type !== 'or') return
    onChange({ type: value.type, children: value.children.map((c, i) => (i === index ? wrapAs(c, kind) : c)) })
  }
  // not 子节点操作(作用于 root.child)。
  const notChildChange = (next: MatchTreeNode) => {
    if (value.type !== 'not') return
    onChange({ type: 'not', child: next })
  }
  const notChildDelete = () => { onChange(newLeaf()) }
  const notChildWrap = (kind: 'and' | 'or' | 'not') => {
    if (value.type !== 'not') return
    onChange({ type: 'not', child: wrapAs(value.child, kind) })
  }

  // not 节点:子节点回调走 notChild*
  if (value.type === 'not') {
    return (
      <MatchNodeRow
        node={value}
        path={[]}
        assetType={assetType}
        readOnly={readOnly}
        onChange={rootChange}
        onWrap={rootWrap}
        onUnwrap={rootUnwrap}
        onDelete={rootDelete}
        onAddChild={() => {}}
        onMoveChild={() => {}}
        onChildChange={(_i, next) => notChildChange(next)}
        onChildDelete={() => notChildDelete()}
        onChildWrap={(_i, k) => notChildWrap(k)}
      />
    )
  }

  return (
    <MatchNodeRow
      node={value}
      path={[]}
      assetType={assetType}
      readOnly={readOnly}
      onChange={rootChange}
      onWrap={rootWrap}
      onUnwrap={rootUnwrap}
      onDelete={rootDelete}
      onAddChild={addChild}
      onMoveChild={moveChildHandler}
      onChildChange={childChange}
      onChildDelete={childDelete}
      onChildWrap={childWrap}
    />
  )
}
