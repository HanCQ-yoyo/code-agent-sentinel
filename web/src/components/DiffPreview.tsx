import { Modal, Alert, Typography } from 'antd'
import type { PreviewResult } from '../types'

// DiffPreview:预览变更 Modal。diff 文本(危险行标红)+ dangerous 列表 + base_hash 状态。
// onConfirm = 确认保存(提交);onCancel = 取消(返回编辑)。
// base_hash_ok=false 时禁用确认按钮(文件已被外部修改,需重新加载)。
export function DiffPreview({
  open,
  preview,
  onConfirm,
  onCancel,
}: {
  open: boolean
  preview: PreviewResult | null
  onConfirm: () => void
  onCancel: () => void
}) {
  if (!preview) return null
  return (
    <Modal
      title="预览变更"
      open={open}
      onOk={onConfirm}
      onCancel={onCancel}
      okText="确认保存"
      cancelText="取消"
      okButtonProps={{ disabled: !preview.base_hash_ok }}
      width={720}
    >
      {!preview.base_hash_ok && (
        <Alert
          type="warning"
          showIcon
          message="文件已被外部修改(编辑期间)"
          description="基准 hash 不符,保存可能覆盖他人改动。请重新加载资产后再编辑。"
          style={{ marginBottom: 12 }}
        />
      )}
      {preview.dangerous.length > 0 && (
        <Alert
          type="error"
          showIcon
          message="检测到危险变更"
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {preview.dangerous.map((d, i) => (
                <li key={i}>
                  <Typography.Text type="danger">[{d.kind}]</Typography.Text> {d.message}(行 {d.line})
                </li>
              ))}
            </ul>
          }
          style={{ marginBottom: 12 }}
        />
      )}
      <pre
        style={{
          background: 'var(--bg-surface, #f5f5f5)',
          padding: 12,
          borderRadius: 4,
          fontSize: 12,
          fontFamily: 'var(--font-mono, monospace)',
          maxHeight: 360,
          overflow: 'auto',
          whiteSpace: 'pre-wrap',
        }}
      >
        {preview.diff || '(无变更)'}
      </pre>
    </Modal>
  )
}
