import { useEffect } from 'react'
import { Layout, Button, Switch, Space, Select, Breadcrumb } from 'antd'
import { ReloadOutlined, HomeOutlined } from '@ant-design/icons'
import { Link, useLocation } from 'react-router-dom'
import { useTheme } from '../theme'
import { useStore } from '../store'
import { agentMeta } from '../lib/agents'
import { navLabels } from '../lib/nav'
import type { Agent, DetectorMeta } from '../types'

const { Header } = Layout

interface Props {
  onScan: () => void
  loading: boolean
  detectors: DetectorMeta[]
}

// 末段面包屑文案(含动态 :id 路由)。父段用 navLabels(侧栏文案单一来源)。
function leafLabel(pathname: string): string | null {
  if (pathname.match(/^\/assets\/[^/]+$/)) return '资产详情'
  if (pathname.match(/^\/history\/[^/]+$/)) return '扫描详情'
  return null
}

export function TopBar({ onScan, loading, detectors }: Props) {
  const { theme, toggle } = useTheme()
  const { agents, fetchAgents } = useStore()
  const loc = useLocation()
  const currentAgent = agents?.current
  const avail = detectors.filter((d) => d.available).length

  // 当前一级路由(用于面包屑首段)。
  const root = loc.pathname === '/' ? '/dashboard' : `/${loc.pathname.split('/')[1]}`
  const rootLabel = navLabels[root]
  const leaf = leafLabel(loc.pathname)

  // agent 加载:移出 render body 防 render 中触发副作用。
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

  return (
    <Header
      style={{
        background: 'var(--bg-card)',
        borderBottom: '1px solid var(--bg-border)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 24px',
        height: 56,
      }}
    >
      <Space size="middle" style={{ flex: 1, minWidth: 0 }}>
        <Breadcrumb items={crumbItems} />
        <Select
          size="small"
          style={{ width: 150 }}
          value={currentAgent ?? (agents?.agents?.[0]?.id ?? undefined)}
          disabled={(agents?.agents?.length ?? 0) <= 1}
          options={(agents?.agents ?? []).map((a: Agent) => ({ value: a.id, label: `${agentMeta(a).icon} ${agentMeta(a).label}` }))}
          onChange={() => { /* 单 agent 本轮无实际切换;未来多 agent 在此 dispatch */ }}
        />
      </Space>
      <Space size="middle">
        <span data-testid="detector-summary" style={{ color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
          检测器 {avail}/{detectors.length}
        </span>
        <Switch
          checked={theme === 'dark'}
          onChange={toggle}
          checkedChildren="深"
          unCheckedChildren="浅"
          aria-label="主题"
        />
        <Button type="primary" icon={<ReloadOutlined />} loading={loading} onClick={onScan}>
          {loading ? '扫描中…' : '重新扫描'}
        </Button>
      </Space>
    </Header>
  )
}
