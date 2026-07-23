import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Card, Button, Spin } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { apiGet } from '../api/client'
import type { Asset } from '../types'
import { AssetDetailPanel } from './AssetDetailPanel'

export default function AssetDetail() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const [asset, setAsset] = useState<Asset | null>(null)
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let stale = false
    setLoading(true)
    apiGet<Asset>(`/api/assets/${id}`)
      .then((a) => { if (!stale) setAsset(a) })
      .catch((e) => { if (!stale) setErr(String(e)) })
      .finally(() => { if (!stale) setLoading(false) })
    return () => { stale = true }
  }, [id])

  if (loading) return <Spin style={{ display: 'block', margin: '40px auto' }} />
  if (err || !asset) return <Card>{err ?? t('assetDetail.notFound')}</Card>
  return (
    <div>
      <Link to="/assets"><Button type="link" icon={<ArrowLeftOutlined />}>{t('assetDetail.back')}</Button></Link>
      {/* 全页路由无 agent 上下文:安全检查不传 agentID → runScan 不带 ?agents= → 后端回退 SelectedAgentID
          (与 getAsset 的 engineForQuery 回退一致)。URL 不含 agent,故依赖后端默认。 */}
      <Card><AssetDetailPanel asset={asset} /></Card>
    </div>
  )
}
