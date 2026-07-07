import { Layout, Button, Switch, Space, Typography } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { useTheme } from '../theme'
import type { DetectorMeta } from '../types'

const { Header } = Layout

interface Props {
  title: string
  onScan: () => void
  loading: boolean
  detectors: DetectorMeta[]
}

export function TopBar({ title, onScan, loading, detectors }: Props) {
  const { theme, toggle } = useTheme()
  const avail = detectors.filter((d) => d.available).length
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
      <Typography.Title level={4} style={{ color: 'var(--text)', margin: 0 }}>
        {title}
      </Typography.Title>
      <Space size="middle">
        <span data-testid="detector-summary" style={{ color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
          检测器 {avail}/{detectors.length}
        </span>
        <Switch
          checked={theme === 'dark'}
          onChange={toggle}
          checkedChildren="深"
          unCheckedChildren="浅"
        />
        <Button type="primary" icon={<ReloadOutlined />} loading={loading} onClick={onScan}>
          {loading ? '扫描中…' : '重新扫描'}
        </Button>
      </Space>
    </Header>
  )
}
