import { useState } from 'react'
import { Button, Space, message } from 'antd'
import { EditOutlined } from '@ant-design/icons'
import { ContentArea, editableText } from './ContentArea'
import { DiffPreview } from './DiffPreview'
import { useStore } from '../store'
import { useTheme } from '../theme'
import type { Asset, PreviewResult } from '../types'

// AssetEditor:资产详情编辑模式容器。只读态显示「编辑」按钮 + ContentArea;
// 编辑态显示工具条(取消/预览变更)+ 可编辑 ContentArea + 预览 Modal。
//
// 编辑流程:enterEdit(快照 editableText 为 draft)→ 用户编辑 → doPreview(后端算 diff + 危险检测)
// → DiffPreview Modal → doCommit(备份 + 原子写 + 部分重扫)→ 反馈新增风险数。
//
// useTheme() 返回 { theme, toggle },取 theme 字段(非对象)传给 ContentArea。
// key={editing ? 'edit' : 'view'}:编辑态切换时强制 ContentArea 重挂载,
// 使 Segmented view state 重置(编辑态默认源码、只读态默认预览)且 Monaco 以新 readOnly 加载。
export function AssetEditor({ asset }: { asset: Asset }) {
  const { theme } = useTheme()
  const { previewAssetEdit, commitAssetEdit } = useStore()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [preview, setPreview] = useState<PreviewResult | null>(null)
  const [previewOpen, setPreviewOpen] = useState(false)
  const [saving, setSaving] = useState(false)

  // 进入编辑:先 preview 探测可编辑性 + 乐观锁(base_hash_ok),
  // 不可编辑或已被外部修改则提示并拒绝进入编辑态,避免给用户一个编辑不了的 UI。
  // draft 仍用 editableText(asset) 快照——structured 资产无 content,
  // 须从 fields.raw 取,否则 draft 为空导致编辑 silently 无效。
  // probe 的 new_content 传空(asset.content ?? '')是安全的:后端 Preview 先跑 editable() 判定,
  // 不可编辑时直接返回 {Editable:false},不计算 diff;可编辑时 enterEdit 也不使用返回的 diff。
  const enterEdit = async () => {
    setSaving(true)
    try {
      const pr = await previewAssetEdit(asset.id, asset.content ?? '', asset.hash)
      if (!pr) return
      if (!pr.editable) {
        message.warning(pr.not_editable_reason || '该资产不可编辑')
        return
      }
      if (!pr.base_hash_ok) {
        message.warning('文件已被外部修改,请重新加载资产后再编辑')
        return
      }
      setDraft(editableText(asset))
      setEditing(true)
    } finally {
      setSaving(false)
    }
  }

  const doPreview = async () => {
    setSaving(true)
    try {
      const pr = await previewAssetEdit(asset.id, draft, asset.hash)
      if (pr) {
        if (!pr.editable) {
          message.warning(`该资产不可编辑:${pr.not_editable_reason ?? '未知原因'}`)
          return
        }
        setPreview(pr)
        setPreviewOpen(true)
      }
    } finally {
      setSaving(false)
    }
  }

  const doCommit = async () => {
    setSaving(true)
    try {
      const res = await commitAssetEdit(asset.id, draft, asset.hash)
      if (res) {
        setPreviewOpen(false)
        setEditing(false)
        const n = (res.new_findings ?? []).length
        if (n > 0) {
          message.warning(`已保存;部分重扫发现 ${n} 项新增风险。可点「重新扫描」做全量。`)
        } else {
          message.success('已保存;部分重扫未发现新增风险。')
        }
      }
    } finally {
      setSaving(false)
    }
  }

  if (!editing) {
    return (
      <>
        <Button icon={<EditOutlined />} onClick={enterEdit} loading={saving} size="small" style={{ marginBottom: 8 }}>
          编辑
        </Button>
        <ContentArea key="view" asset={asset} theme={theme} />
      </>
    )
  }

  return (
    <>
      <Space style={{ marginBottom: 8 }}>
        <Button onClick={() => setEditing(false)} disabled={saving}>
          取消
        </Button>
        <Button type="primary" onClick={doPreview} loading={saving} data-testid="preview-edit">
          预览变更
        </Button>
      </Space>
      <ContentArea
        key="edit"
        asset={{ ...asset, content: draft }}
        theme={theme}
        readOnly={false}
        onChange={setDraft}
      />
      <DiffPreview
        open={previewOpen}
        preview={preview}
        onConfirm={doCommit}
        onCancel={() => setPreviewOpen(false)}
      />
    </>
  )
}
