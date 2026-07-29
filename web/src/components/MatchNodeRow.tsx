import { AutoComplete, Select, Input, Tag, Dropdown, Button, Space, Tooltip } from 'antd'
import { PlusOutlined, DeleteOutlined, ArrowUpOutlined, ArrowDownOutlined, DownOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import type { MatchTreeNode } from '../lib/match-tree'
import { newLeaf, wrapAs, moveChild } from '../lib/match-tree'
import { fieldSuggestions } from '../lib/asset-fields'
import type { MenuProps } from 'antd'

// op 分 4 组(对齐 validate.go value 契约)。
const OP_GROUPS: { groupKey: string; ops: { value: string }[] }[] = [
  { groupKey: 'ruleForm.opGroup.existence', ops: [{ value: 'exists' }, { value: 'not_exists' }] },
  { groupKey: 'ruleForm.opGroup.set', ops: [{ value: 'within' }, { value: 'not_within' }] },
  { groupKey: 'ruleForm.opGroup.text', ops: [{ value: 'eq' }, { value: 'not_equals' }, { value: 'contains' }, { value: 'not_contains' }] },
  { groupKey: 'ruleForm.opGroup.regex', ops: [{ value: 'regex_match' }, { value: 'not_regex_match' }, { value: 'key_matches' }] },
]
const ARRAY_OPS = new Set(['within', 'not_within'])
const NO_VALUE_OPS = new Set(['exists', 'not_exists'])
const REGEX_OPS = new Set(['regex_match', 'not_regex_match', 'key_matches'])

export interface MatchNodeRowProps {
  node: MatchTreeNode
  path: number[]
  assetType: string
  readOnly: boolean
  onChange: (next: MatchTreeNode) => void
  onWrap: (kind: 'and' | 'or' | 'not') => void
  onUnwrap: (kind: 'and' | 'or' | 'leaf') => void
  onDelete: () => void
  onAddChild: (kind: 'leaf' | 'group') => void
  onMoveChild: (index: number, dir: 'up' | 'down') => void
  onChildChange: (index: number, next: MatchTreeNode) => void
  onChildDelete: (index: number) => void
  onChildWrap: (index: number, kind: 'and' | 'or' | 'not') => void
}

// 节点变换菜单项(叶子可转 and/or/not;and/or 可转 not;not 可转 and/or/leaf)。
function transformMenuItems(node: MatchTreeNode, t: (k: string) => string, onWrap: (k: 'and' | 'or' | 'not') => void, onUnwrap: (k: 'and' | 'or' | 'leaf') => void): MenuProps['items'] {
  const items: NonNullable<MenuProps['items']> = []
  if (node.type === 'leaf') {
    items.push({ key: 'and', label: t('ruleForm.convertToAnd'), onClick: () => onWrap('and') })
    items.push({ key: 'or', label: t('ruleForm.convertToOr'), onClick: () => onWrap('or') })
    items.push({ key: 'not', label: t('ruleForm.convertToNot'), onClick: () => onWrap('not') })
  } else if (node.type === 'and' || node.type === 'or') {
    items.push({ key: 'not', label: t('ruleForm.convertToNot'), onClick: () => onWrap('not') })
  } else {
    items.push({ key: 'and', label: t('ruleForm.convertToAnd'), onClick: () => onUnwrap('and') })
    items.push({ key: 'or', label: t('ruleForm.convertToOr'), onClick: () => onUnwrap('or') })
    items.push({ key: 'leaf', label: t('ruleForm.unwrapToLeaf'), onClick: () => onUnwrap('leaf') })
  }
  return items
}

// 叶子紧凑表单:field AutoComplete + op 分组 Select + value 动态控件。
function LeafFields({ node, assetType, readOnly, onChange, t }: {
  node: Extract<MatchTreeNode, { type: 'leaf' }>
  assetType: string
  readOnly: boolean
  onChange: (next: MatchTreeNode) => void
  t: (k: string) => string
}) {
  const op = node.op
  const suggestions = fieldSuggestions(assetType).map((f) => ({ value: f }))

  // value 控件按 op 契约切换。
  let valueControl: React.ReactNode = null
  if (!NO_VALUE_OPS.has(op) && op !== '') {
    if (ARRAY_OPS.has(op)) {
      // 数组 value:Select tags
      const arr = Array.isArray(node.value) ? node.value : []
      valueControl = (
        <Select
          mode="tags"
          style={{ minWidth: 180 }}
          disabled={readOnly}
          placeholder={t('ruleForm.valuePlaceholder')}
          value={arr}
          onChange={(v: string[]) => onChange({ ...node, value: v })}
        />
      )
    } else {
      const isRegex = REGEX_OPS.has(op)
      valueControl = (
        <Input
          style={{ minWidth: 200 }}
          disabled={readOnly}
          placeholder={isRegex ? t('ruleForm.regexPlaceholder') : t('ruleForm.valuePlaceholder')}
          value={Array.isArray(node.value) ? '' : node.value}
          onChange={(e) => onChange({ ...node, value: e.target.value })}
          suffix={isRegex ? (
            <Tooltip title={t('ruleForm.dotallHint')}><span style={{ cursor: 'help' }}>(?s)</span></Tooltip>
          ) : null}
        />
      )
    }
  }

  return (
    <Space.Compact style={{ width: '100%' }} block>
      <AutoComplete
        style={{ minWidth: 140 }}
        disabled={readOnly}
        placeholder={t('ruleForm.fieldPlaceholder')}
        value={node.field}
        options={suggestions}
        onChange={(v: string) => onChange({ ...node, field: v })}
        filterOption={(input, option) => (option?.value ?? '').toLowerCase().includes(input.toLowerCase())}
      />
      <Select
        style={{ minWidth: 140 }}
        disabled={readOnly}
        placeholder={t('ruleForm.opLabel')}
        value={op || undefined}
        onChange={(v: string) => {
          // 切 op 时按新契约重置 value:within/not_within→[];其余(exists/not_exists/标量/正则)→''
          const value: string | string[] = ARRAY_OPS.has(v) ? [] : ''
          onChange({ ...node, op: v, value })
        }}
      >
        {OP_GROUPS.map((g) => (
          <Select.OptGroup key={g.groupKey} label={t(g.groupKey)}>
            {g.ops.map((o) => <Select.Option key={o.value} value={o.value}>{o.value}</Select.Option>)}
          </Select.OptGroup>
        ))}
      </Select>
      {valueControl}
    </Space.Compact>
  )
}

export function MatchNodeRow({
  node, assetType, readOnly, onChange, onWrap, onUnwrap, onDelete,
  onAddChild, onMoveChild, onChildChange, onChildDelete, onChildWrap,
}: MatchNodeRowProps) {
  const { t } = useTranslation()

  // 分组标题行(AND/OR/NOT)。用 !== 'leaf' 守卫:联合里 and/or 合并成一个成员
  // { type: 'and' | 'or' },复合 === 'and' || === 'or' || === 'not' 在 TS 5.9 strict 下
  // 不会穷尽收窄掉该成员,后续 LeafFields 仍以为 node 可能是 and|or → TS2322。
  // 否定守卫 !== 'leaf' TS 能可靠收窄到 leaf,运行时等价(联合只有 leaf / 非叶子两类)。
  if (node.type !== 'leaf') {
    const label = node.type.toUpperCase()
    const children = node.type === 'not' ? [node.child] : node.children
    return (
      <div style={{ borderLeft: '2px solid var(--border)', paddingLeft: 12, marginBottom: 8 }}>
        <Space style={{ marginBottom: 6 }}>
          <Tag color={node.type === 'not' ? 'orange' : 'blue'}>{label}</Tag>
          {!readOnly && (node.type === 'and' || node.type === 'or') && (
            <>
              <Button size="small" icon={<PlusOutlined />} onClick={() => onAddChild('leaf')}>{t('ruleForm.addCondition')}</Button>
              <Button size="small" icon={<PlusOutlined />} onClick={() => onAddChild('group')}>{t('ruleForm.addGroup')}</Button>
            </>
          )}
          {!readOnly && (
            <Dropdown menu={{ items: transformMenuItems(node, t, onWrap, onUnwrap) }}>
              <Button size="small" type="text" icon={<DownOutlined />} />
            </Dropdown>
          )}
          {!readOnly && <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={onDelete} />}
        </Space>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {children.map((child, i) => (
            <div key={i} style={{ display: 'flex', alignItems: 'flex-start', gap: 4 }}>
              <MatchNodeRow
                node={child}
                path={[]}
                assetType={assetType}
                readOnly={readOnly}
                onChange={(next) => node.type === 'not' ? onChildChange(0, next) : onChildChange(i, next)}
                onWrap={(k) => node.type === 'not' ? onChildWrap(0, k) : onChildWrap(i, k)}
                onUnwrap={() => {}}
                onDelete={() => node.type === 'not' ? onChildDelete(0) : onChildDelete(i)}
                onAddChild={(ck) => {
                  // 子层加子节点:把子层变换后整体冒泡
                  if (child.type === 'and' || child.type === 'or') {
                    const newChild: MatchTreeNode = ck === 'leaf' ? newLeaf() : { type: 'and', children: [newLeaf()] }
                    const next = { ...child, children: [...child.children, newChild] } as MatchTreeNode
                    node.type === 'not' ? onChildChange(0, next) : onChildChange(i, next)
                  }
                }}
                onMoveChild={(ci, dir) => {
                  if (child.type === 'and' || child.type === 'or') {
                    const next = moveChild(child, ci, dir)
                    node.type === 'not' ? onChildChange(0, next) : onChildChange(i, next)
                  }
                }}
                onChildChange={(ci, next) => {
                  if (child.type === 'and' || child.type === 'or') {
                    const updated = { ...child, children: child.children.map((c, j) => (j === ci ? next : c)) } as MatchTreeNode
                    node.type === 'not' ? onChildChange(0, updated) : onChildChange(i, updated)
                  }
                }}
                onChildDelete={(ci) => {
                  if (child.type === 'and' || child.type === 'or') {
                    const children = child.children.filter((_, j) => j !== ci)
                    const next: MatchTreeNode = children.length === 0 ? newLeaf() : { ...child, children } as MatchTreeNode
                    node.type === 'not' ? onChildChange(0, next) : onChildChange(i, next)
                  }
                }}
                onChildWrap={(ci, k) => {
                  if (child.type === 'and' || child.type === 'or') {
                    const updated = { ...child, children: child.children.map((c, j) => (j === ci ? wrapAs(c, k) : c)) } as MatchTreeNode
                    node.type === 'not' ? onChildChange(0, updated) : onChildChange(i, updated)
                  }
                }}
              />
              {!readOnly && node.type !== 'not' && (
                <Space direction="vertical" size={0}>
                  <Button size="small" type="text" icon={<ArrowUpOutlined />} onClick={() => onMoveChild(i, 'up')} />
                  <Button size="small" type="text" icon={<ArrowDownOutlined />} onClick={() => onMoveChild(i, 'down')} />
                </Space>
              )}
            </div>
          ))}
        </div>
      </div>
    )
  }

  // 叶子行
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
      <LeafFields node={node} assetType={assetType} readOnly={readOnly} onChange={onChange} t={t} />
      {!readOnly && (
        <>
          <Dropdown menu={{ items: transformMenuItems(node, t, onWrap, onUnwrap) }}>
            <Button size="small" type="text" icon={<DownOutlined />} />
          </Dropdown>
          <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={onDelete} />
        </>
      )}
    </div>
  )
}
