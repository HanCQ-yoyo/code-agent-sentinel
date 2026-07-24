import { useState } from 'react'
import { Button, message, Modal } from 'antd'
import { EditOutlined, FullscreenOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { ContentArea, editableText } from './ContentArea'
import { DiffPreview } from './DiffPreview'
import { useStore } from '../store'
import { useTheme } from '../theme'
import type { Asset, PreviewResult } from '../types'

// AssetEditor:资产详情编辑模式容器。只读态显示「编辑」+「全屏」按钮 + ContentArea;
// 编辑态显示工具条(取消/预览变更)+ 可编辑 ContentArea + 预览 Modal。
//
// 编辑流程:enterEdit(快照 editableText 为 draft)→ 用户编辑 → doPreview(后端算 diff + 危险检测)
// → DiffPreview Modal → doCommit(备份 + 原子写 + 部分重扫)→ 反馈新增风险数。
//
// 全屏:只读态点「全屏」开一个近全屏 Modal,内部复用 ContentArea(只读),在完整抽屉页
// 展示资产内容。Markdown 预览/源码切换、Monaco 滚动等行为与内联一致,仅放大展示空间。
//
// useTheme() 返回 { theme, toggle },取 theme 字段(非对象)传给 ContentArea。
// key={editing ? 'edit' : 'view'}:编辑态切换时强制 ContentArea 重挂载,
// 使 Segmented view state 重置(编辑态默认源码、只读态默认预览)且 Monaco 以新 readOnly 加载。
export function AssetEditor({ asset, highlights }: { asset: Asset, highlights?: { line: number; startCol: number; endCol: number }[] }) {
  const { t } = useTranslation()
  const { theme } = useTheme()
  const { previewAssetEdit, commitAssetEdit } = useStore()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [preview, setPreview] = useState<PreviewResult | null>(null)
  const [previewOpen, setPreviewOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  // 全屏 Modal:只读态可用;editing 时隐藏入口(编辑态聚焦内联编辑,避免两处可编辑态)。
  const [fsOpen, setFsOpen] = useState(false)

  // 进入编辑:先 preview 探测可编辑性 + 乐观锁(base_hash_ok),
  // 不可编辑或已被外部修改则提示并拒绝进入编辑态,避免给用户一个编辑不了的 UI。
  // draft 从 pr.original_content 初始化——后端直接读源文件的原始磁盘内容,
  // 保证编辑起点 = 真实文件内容(对所有资产类型一致)。
  // structured 资产(settings/permissions/hooks/mcp_server/keybinding)的 fields.raw
  // 是 json.RawMessage(marshal 为对象)或根本没有 raw 字段,用 JSON.stringify(fields)
  // 会导致 draft 是整个 fields 包装而非文件内容 → commit 损坏文件。
  // editableText(asset) 仅作为 original_content 缺失时的防御性回退。
  // probe 的 new_content 传空(asset.content ?? '')是安全的:后端 Preview 先跑 editable() 判定,
  // 不可编辑时直接返回 {Editable:false},不计算 diff;可编辑时 enterEdit 也不使用返回的 diff。
  const enterEdit = async () => {
    setSaving(true)
    try {
      const pr = await previewAssetEdit(asset.id, asset.content ?? '', asset.hash)
      if (!pr) return
      if (!pr.editable) {
        message.warning(pr.not_editable_reason || t('assetEditor.notEditable'))
        return
      }
      if (!pr.base_hash_ok) {
        message.warning(t('assetEditor.hashMismatch'))
        return
      }
      setDraft(pr.original_content ?? editableText(asset))
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
          message.warning(pr.not_editable_reason ? t('assetEditor.previewNotEditable', { reason: pr.not_editable_reason }) : t('assetEditor.previewNotEditableUnknown'))
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
        if (res.rescan_error) {
          message.warning(t('assetEditor.savedWithRescanError', { error: res.rescan_error }))
          return
        }
        const n = (res.new_findings ?? []).length
        if (n > 0) {
          message.warning(t('assetEditor.savedNewFindings', { count: n }))
        } else {
          message.success(t('assetEditor.savedNoNewFindings'))
        }
      }
    } finally {
      setSaving(false)
    }
  }

  // 内容框标题区操作按钮(只读态:编辑 + 全屏;编辑态:取消 + 预览变更)。
  // 由 ContentArea 的 headerActions 渲染在 Card extra(预览/源码 Segmented 左侧)——把编辑/全屏
  // 收进内容框,而非浮在内容框上方独立一行(用户需求:编辑/全屏置于预览/源码左侧)。
  const headerActions = (
    <>
      {!editing ? (
        <>
          <Button icon={<EditOutlined />} onClick={enterEdit} loading={saving} size="small">
            {t('assetEditor.edit')}
          </Button>
          <Button icon={<FullscreenOutlined />} onClick={() => setFsOpen(true)} size="small">
            {t('assetEditor.fullscreen')}
          </Button>
        </>
      ) : (
        <>
          <Button onClick={() => setEditing(false)} disabled={saving} size="small">
            {t('assetEditor.cancel')}
          </Button>
          <Button type="primary" onClick={doPreview} loading={saving} size="small" data-testid="preview-edit">
            {t('assetEditor.previewChange')}
          </Button>
        </>
      )}
    </>
  )

  if (!editing) {
    return (
      <>
        <ContentArea key="view" asset={asset} theme={theme} highlights={highlights} headerActions={headerActions} />
        {/* 全屏 Modal:近全屏(宽 96vw / 高 92vh),内部只读 ContentArea 撑满。
            key={asset.id}:切资产时重挂载,使 ContentArea 的 Segmented view 回默认(预览),
            避免上一资产的全屏视图态泄漏。body 无 padding,ContentArea 自带 Card 内边距。
            destroyOnClose:关闭后卸载内嵌 Monaco,释放编辑器实例。
            全屏内复用 ContentArea 不再传 headerActions(全屏内无需编辑/全屏按钮)。 */}
        <Modal
          title={t('assetEditor.title', { name: asset.name })}
          open={fsOpen}
          onCancel={() => setFsOpen(false)}
          footer={null}
          width="96vw"
          centered
          styles={{ body: { height: 'calc(88vh - 55px)', padding: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column' } }}
          destroyOnClose
        >
          {fsOpen ? (
            <ContentArea key={asset.id} asset={asset} theme={theme} highlights={highlights} fill />
          ) : null}
        </Modal>
      </>
    )
  }

  return (
    <>
      <ContentArea
        key="edit"
        asset={{ ...asset, content: draft }}
        theme={theme}
        readOnly={false}
        onChange={setDraft}
        highlights={highlights}
        headerActions={headerActions}
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
