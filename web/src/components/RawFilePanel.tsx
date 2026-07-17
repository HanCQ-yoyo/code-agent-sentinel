import { lazy, Suspense, useEffect, useMemo, useState } from 'react'
import { Card, Spin, Alert, Descriptions, Typography, Empty, Segmented } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { useTheme } from '../theme'
import { langByExt } from '../lib/monaco-lang'
import { MarkdownPreview } from './MarkdownPreview'
import type { RawFile } from '../types'

const MonacoViewer = lazy(() => import('./MonacoViewer'))

// RawFilePanel:展示「无资产」文件的原始内容(文件树点击非配置资产文件时)。
// 经 /api/raw 读取(后端校验路径必须在树根之下,防越权)。
// .md 文本:预览/源码 切换(预览走 MarkdownPreview,与资产 .md 体验一致)。
// .json 文本:自动格式化(JSON.parse + stringify 缩进 2,失败回退原文)。
// 其余文本:Monaco 按扩展名高亮;二进制/超大显示提示。
export function RawFilePanel({ path }: { path: string }) {
  const { t } = useTranslation()
  const { theme } = useTheme()
  const fetchRaw = useStore((s) => s.fetchRaw)
  const [data, setData] = useState<RawFile | null>(null)
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)
  // .md 文件默认预览;切源码看原始文本。
  const [mdView, setMdView] = useState<'preview' | 'source'>('preview')

  useEffect(() => {
    let stale = false
    setLoading(true)
    setErr(null)
    setData(null)
    setMdView('preview')
    fetchRaw(path)
      .then((r) => { if (!stale) setData(r ?? null) })
      .catch((e) => { if (!stale) setErr(String(e)) })
      .finally(() => { if (!stale) setLoading(false) })
    return () => { stale = true }
  }, [path, fetchRaw])

  const lang = data ? langByExt(data.path) : ''
  const isMarkdown = lang === 'markdown'
  // .json 文本:尝试格式化,失败(非法 JSON)回退原文(仍带语法高亮)。
  const displayContent = useMemo(() => {
    if (!data || !data.is_text) return ''
    if (lang === 'json' && data.content) {
      try {
        return JSON.stringify(JSON.parse(data.content), null, 2)
      } catch {
        return data.content // 非法 JSON:原文展示,不吞错
      }
    }
    return data.content
  }, [data, lang])

  if (loading) return <Spin style={{ display: 'block', margin: '40px auto' }} />
  if (err) return <Alert type="error" message={t('rawFile.loadFailed')} description={err} showIcon />
  if (!data) return <Empty description={t('rawFile.notFound')} />

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, height: '100%' }}>
      <div>
        <h2 data-testid="raw-file-name" style={{ color: 'var(--text)', margin: '0 0 4px', fontFamily: 'var(--font-mono)', fontSize: 18 }}>{data.name}</h2>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {t('rawFile.subtitle')}
        </Typography.Text>
      </div>
      <Descriptions size="small" column={2} bordered>
        <Descriptions.Item label={t('rawFile.path')} span={2}>
          <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{data.path}</Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label={t('rawFile.size')}>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{formatSize(data.size)}</span>
        </Descriptions.Item>
        <Descriptions.Item label={t('rawFile.type')}>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{data.is_text ? t('rawFile.typeText') : t('rawFile.typeBinary')}</span>
        </Descriptions.Item>
      </Descriptions>
      <Card
        size="small"
        title={t('rawFile.content')}
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: isMarkdown && mdView === 'preview' ? 0 : 12, overflow: 'hidden', display: 'flex', flexDirection: 'column' } }}
        extra={isMarkdown ? (
          <Segmented
            size="small"
            value={mdView}
            onChange={(v) => setMdView(v as 'preview' | 'source')}
            options={[{ value: 'preview', label: t('rawFile.preview') }, { value: 'source', label: t('rawFile.source') }]}
          />
        ) : undefined}
      >
        {data.is_text ? (
          isMarkdown && mdView === 'preview' ? (
            <div style={{ padding: 12, flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column' }}>
              <MarkdownPreview content={data.content} />
            </div>
          ) : (
            <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
              <MonacoViewer value={displayContent} language={lang} theme={theme} />
            </Suspense>
          )
        ) : (
          <Empty description={data.content} />
        )}
      </Card>
    </div>
  )
}

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}
