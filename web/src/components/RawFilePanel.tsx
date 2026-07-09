import { lazy, Suspense, useEffect, useState } from 'react'
import { Card, Spin, Alert, Descriptions, Typography, Empty } from 'antd'
import { useStore } from '../store'
import { useTheme } from '../theme'
import { langByExt } from '../lib/monaco-lang'
import type { RawFile } from '../types'

const MonacoViewer = lazy(() => import('./MonacoViewer'))

// RawFilePanel:展示「无资产」文件的原始内容(文件树点击非配置资产文件时)。
// 经 /api/raw 读取(后端校验路径必须在树根之下,防越权)。
// 文本文件用 Monaco(按扩展名选语言);二进制/超大显示提示。
export function RawFilePanel({ path }: { path: string }) {
  const { theme } = useTheme()
  const fetchRaw = useStore((s) => s.fetchRaw)
  const [data, setData] = useState<RawFile | null>(null)
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let stale = false
    setLoading(true)
    setErr(null)
    setData(null)
    fetchRaw(path)
      .then((r) => { if (!stale) setData(r ?? null) })
      .catch((e) => { if (!stale) setErr(String(e)) })
      .finally(() => { if (!stale) setLoading(false) })
    return () => { stale = true }
  }, [path, fetchRaw])

  if (loading) return <Spin style={{ display: 'block', margin: '40px auto' }} />
  if (err) return <Alert type="error" message="读取失败" description={err} showIcon />
  if (!data) return <Empty description="无法读取该文件" />

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, height: '100%' }}>
      <div>
        <h2 data-testid="raw-file-name" style={{ color: 'var(--text)', margin: '0 0 4px', fontFamily: 'var(--font-mono)', fontSize: 18 }}>{data.name}</h2>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          运行时/非配置文件(只读原始内容)
        </Typography.Text>
      </div>
      <Descriptions size="small" column={2} bordered>
        <Descriptions.Item label="路径" span={2}>
          <Typography.Text code style={{ fontFamily: 'var(--font-mono)', fontSize: 12, wordBreak: 'break-all' }}>{data.path}</Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label="大小">
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{formatSize(data.size)}</span>
        </Descriptions.Item>
        <Descriptions.Item label="类型">
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{data.is_text ? '文本' : '二进制'}</span>
        </Descriptions.Item>
      </Descriptions>
      <Card
        size="small"
        title="内容"
        style={{ flex: 1, minHeight: 240, display: 'flex', flexDirection: 'column' }}
        styles={{ body: { flex: 1, padding: 12, overflow: 'hidden' } }}
      >
        {data.is_text ? (
          <Suspense fallback={<Spin style={{ display: 'block', margin: '40px auto' }} />}>
            <MonacoViewer value={data.content} language={langByExt(data.path)} theme={theme} />
          </Suspense>
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
