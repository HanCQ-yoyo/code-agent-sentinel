import { useEffect, useState } from 'react'
import { Card, Segmented, Input, Radio, Spin, Alert, Typography } from 'antd'
import { useStore } from '../store'
import type { Asset } from '../types'
import { AssetTable } from '../components/AssetTable'
import { AssetTree } from '../components/AssetTree'
import { AssetDrawer } from '../components/AssetDrawer'
import { AssetDetailPanel } from '../components/AssetDetailPanel'

type View = 'list' | 'tree'

export default function Assets() {
  const { assets, fetchAssets, scan, error } = useStore()
  const [view, setView] = useState<View>('list')
  const [type, setType] = useState('')
  const [q, setQ] = useState('')
  const [selected, setSelected] = useState<string | null>(null)

  useEffect(() => { fetchAssets() }, [fetchAssets])

  if (!assets) return <Spin style={{ display: 'block', margin: '40px auto' }} />
  const all = assets.assets
  const types = [...new Set(all.map((a) => a.type))].sort()
  const ql = q.toLowerCase()
  const list = all.filter((a) =>
    (type === '' || a.type === type) &&
    (q === '' || a.name.toLowerCase().includes(ql) || a.source_path.toLowerCase().includes(ql))
  )
  const selectedAsset: Asset | undefined = selected ? all.find((a) => a.id === selected) : undefined

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {error ? <Alert type="error" message="加载失败" description={error} showIcon /> : null}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <Segmented value={view} onChange={(v) => setView(v as View)} options={[{ value: 'list', label: '列表' }, { value: 'tree', label: '文件树' }]} />
        {view === 'list' ? (
          <>
            <Radio.Group value={type} onChange={(e) => setType(e.target.value)} size="small">
              <Radio.Button value="">全部</Radio.Button>
              {types.map((t) => <Radio.Button key={t} value={t}>{t}</Radio.Button>)}
            </Radio.Group>
            <Input.Search value={q} onChange={(e) => setQ(e.target.value)} placeholder="搜索名称或路径" style={{ width: 240, marginLeft: 'auto' }} allowClear />
          </>
        ) : null}
        <Typography.Text type="secondary" style={{ fontFamily: 'var(--font-mono)' }}>
          {view === 'list' ? `${list.length} / ${all.length} 资产` : `${all.length} 资产`}
        </Typography.Text>
      </div>

      {view === 'list' ? (
        <Card>
          <AssetTable assets={list} findings={scan?.findings} onSelect={setSelected} />
          <AssetDrawer asset={selectedAsset ?? null} onClose={() => setSelected(null)} />
        </Card>
      ) : (
        <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
          <Card style={{ flex: 1 }}>
            <AssetTree assets={all} findings={scan?.findings} onSelect={setSelected} />
          </Card>
          <Card style={{ width: 480, position: 'sticky', top: 16, maxHeight: '80vh', overflow: 'auto' }}>
            {selectedAsset ? <AssetDetailPanel asset={selectedAsset} /> : <Typography.Text type="secondary">选择左侧文件树中的资产查看详情</Typography.Text>}
          </Card>
        </div>
      )}
    </div>
  )
}
