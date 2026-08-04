import { useEffect, useState } from 'react'
import { Layout, Button, Space, Breadcrumb, Popover, Progress, Typography } from 'antd'
import { ReloadOutlined, LoadingOutlined, HomeOutlined } from '@ant-design/icons'
import { Link, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { navLabels } from '../lib/nav'
import { GuardCapsule } from './GuardCapsule'

const { Header } = Layout

// 末段面包屑文案(含动态 :id 路由)。父段用 navLabels(侧栏文案单一来源)。
function leafLabel(pathname: string, t: (k: string) => string): string | null {
  if (pathname.match(/^\/assets\/[^/]+$/)) return t('topbar.leafAsset')
  if (pathname.match(/^\/history\/[^/]+$/)) return t('topbar.leafScan')
  return null
}

export function TopBar() {
  const { t } = useTranslation()
  const { agents, fetchAgents, scanTaskBatchId, scanTaskProgress, cancelScan, openRescan } = useStore()
  const loc = useLocation()
  const [popoverOpen, setPopoverOpen] = useState(false)

  // 当前一级路由(用于面包屑首段)。navLabels 存 i18n key,需 t() 翻译。
  const root = loc.pathname === '/' ? '/dashboard' : `/${loc.pathname.split('/')[1]}`
  const rootLabel = navLabels[root] ? t(navLabels[root]) : undefined
  const leaf = leafLabel(loc.pathname, t)

  // agent 加载:TopBar 仍是首次 agent 列表拉取入口。
  useEffect(() => {
    if (!agents) fetchAgents()
  }, [agents, fetchAgents])

  // 面包屑项:🏠 首页 → 当前页(若有 leaf 则加中间段)。
  const crumbItems = [
    { title: <Link to="/dashboard"><HomeOutlined /></Link> },
    leaf
      ? { title: <Link to={root}>{rootLabel}</Link> }
      : { title: <span>{rootLabel}</span> },
  ]
  if (leaf) crumbItems.push({ title: <span>{leaf}</span> })

  const isScanning = !!scanTaskBatchId
  const progressText = scanTaskProgress
    ? t('topbar.scanProgress', { current: scanTaskProgress.current_agent, completed: scanTaskProgress.completed, total: scanTaskProgress.total })
    : ''
  const progressPercent = scanTaskProgress && scanTaskProgress.total > 0
    ? Math.round((scanTaskProgress.completed / scanTaskProgress.total) * 100)
    : 0

  // 扫描按钮
  const scanBtn = isScanning ? (
    <Button
      icon={<LoadingOutlined />}
      style={{ whiteSpace: 'nowrap' }}
      onClick={() => setPopoverOpen(!popoverOpen)}
    >
      {t('topbar.scanning')}
    </Button>
  ) : (
    <Button
      icon={<ReloadOutlined />}
      onClick={() => openRescan()}
      style={{ whiteSpace: 'nowrap' }}
    >
      {t('topbar.rescan')}
    </Button>
  )

  // Popover 内容
  const popoverContent = (
    <div style={{ minWidth: 220 }}>
      <Typography.Text style={{ fontSize: 'var(--fs-sm)', display: 'block', marginBottom: 8 }}>
        {progressText}
      </Typography.Text>
      <Progress percent={progressPercent} size="small" style={{ marginBottom: 12 }} />
      <Button size="small" block onClick={() => { cancelScan(); setPopoverOpen(false) }}>
        {t('topbar.cancelScan')}
      </Button>
    </div>
  )

  return (
    <Header
      style={{
        background: 'var(--color-paper-2)',
        borderBottom: '1px solid var(--color-rule)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 var(--space-2xl)',
        height: 56,
      }}
    >
      <Space size="middle" style={{ flex: 1, minWidth: 0 }}>
        <Breadcrumb items={crumbItems} />
      </Space>
      <Space size="middle">
        <GuardCapsule />
        {isScanning ? (
          <Popover
            open={popoverOpen}
            onOpenChange={setPopoverOpen}
            trigger="click"
            placement="bottomRight"
            content={popoverContent}
          >
            {scanBtn}
          </Popover>
        ) : (
          scanBtn
        )}
      </Space>
    </Header>
  )
}
