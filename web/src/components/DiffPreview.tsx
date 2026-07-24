import { Modal, Alert, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
  if (!preview) return null
  return (
    <Modal
      title={t('diff.title')}
      open={open}
      onOk={onConfirm}
      onCancel={onCancel}
      okText={t('diff.confirm')}
      cancelText={t('diff.cancel')}
      okButtonProps={{ disabled: !preview.base_hash_ok }}
      width={720}
    >
      {!preview.base_hash_ok && (
        <Alert
          type="warning"
          showIcon
          message={t('diff.hashChanged')}
          description={t('diff.hashChangedDesc')}
          style={{ marginBottom: 12 }}
        />
      )}
      {(preview.dangerous ?? []).length > 0 && (
        <Alert
          type="error"
          showIcon
          message={t('diff.dangerous')}
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {(preview.dangerous ?? []).map((d, i) => (
                <li key={i}>
                  <Typography.Text type="danger">[{d.kind}]</Typography.Text> {d.message}{t('diff.line', { line: d.line })}
                </li>
              ))}
            </ul>
          }
          style={{ marginBottom: 12 }}
        />
      )}
      <pre
        style={{
          background: 'var(--color-surface)',
          padding: 'var(--space-md)',
          borderRadius: 'var(--radius-sm)',
          fontSize: 'var(--fs-sm)',
          fontFamily: 'var(--font-mono)',
          fontVariantNumeric: 'tabular-nums',
          maxHeight: 360,
          overflow: 'auto',
          whiteSpace: 'pre-wrap',
        }}
      >
        {preview.diff || t('diff.noChange')}
      </pre>
    </Modal>
  )
}
