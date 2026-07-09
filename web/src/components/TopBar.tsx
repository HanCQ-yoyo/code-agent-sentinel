import { useEffect } from 'react'
import { Layout, Button, Switch, Space, Typography, Select } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { useTheme } from '../theme'
import { useStore } from '../store'
import { agentMeta } from '../lib/agents'
import type { Agent, DetectorMeta } from '../types'

const { Header } = Layout

interface Props {
  title: string
  onScan: () => void
  loading: boolean
  detectors: DetectorMeta[]
}

export function TopBar({ title, onScan, loading, detectors }: Props) {
  const { theme, toggle } = useTheme()
  const { agents, fetchAgents } = useStore()
  const currentAgent = agents?.current
  const avail = detectors.filter((d) => d.available).length

  // agent 加载:移出 render body 防 render 中触发副作用。
  useEffect(() => {
    if (!agents) fetchAgents()
  }, [agents, fetchAgents])

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
      <Space size="middle">
        <Typography.Title level={4} style={{ color: 'var(--text)', margin: 0 }}>
          {title}
        </Typography.Title>
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
