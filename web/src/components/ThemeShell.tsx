import { useState } from 'react'
import { Layout, Menu, Table, Drawer, Switch, Button, Tag, Typography, Space } from 'antd'
import {
  DashboardOutlined,
  AppstoreOutlined,
  WarningOutlined,
  ClockCircleOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import { useTheme } from '../theme'

const { Sider, Header, Content } = Layout

const columns = [
  { title: '名称', dataIndex: 'name', width: '30%' },
  { title: '类型', dataIndex: 'type', width: 110, render: (v: string) => <Tag>{v}</Tag> },
  { title: 'scope', dataIndex: 'scope', width: 90, render: (v: string) => <Tag color="blue">{v}</Tag> },
  { title: '风险', dataIndex: 'risk', width: 96, render: (v?: string) => v ? <Tag color="error">{v}</Tag> : <Tag style={{ borderStyle: 'dashed' }}>无</Tag> },
  { title: '路径', dataIndex: 'path', render: (v: string) => <Typography.Text code>{v}</Typography.Text> },
]

const data = [
  { key: '1', name: 'settings.json', type: 'settings', scope: 'global', risk: 'high', path: 'settings.json' },
  { key: '2', name: 'CLAUDE.md', type: 'memory', scope: 'global', path: 'CLAUDE.md' },
  { key: '3', name: 'summarize.md', type: 'skill', scope: 'project', risk: 'low', path: 'skills/summarize.md' },
]

export function ThemeShell() {
  const { theme, toggle } = useTheme()
  const [open, setOpen] = useState(false)
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible>
        <div style={{ color: 'var(--text)', padding: 16, fontWeight: 600 }}>Sentinel</div>
        <Menu
          theme="dark"
          mode="inline"
          items={[
            { key: 'dashboard', icon: <DashboardOutlined />, label: '看板' },
            { key: 'assets', icon: <AppstoreOutlined />, label: '资产' },
            { key: 'findings', icon: <WarningOutlined />, label: '发现' },
            { key: 'history', icon: <ClockCircleOutlined />, label: '历史' },
            { key: 'settings', icon: <SettingOutlined />, label: '设置' },
          ]}
        />
      </Sider>
      <Layout>
        <Header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 24px' }}>
          <h1 style={{ color: 'var(--text)', margin: 0, fontSize: 18 }}>主题空壳验证</h1>
          <Space>
            <span style={{ color: 'var(--text-muted)' }}>{theme === 'dark' ? '深色' : '浅色'}</span>
            <Switch checked={theme === 'dark'} onChange={toggle} />
            <Button onClick={() => setOpen(true)}>开抽屉</Button>
          </Space>
        </Header>
        <Content style={{ padding: 24 }}>
          <Table columns={columns} dataSource={data} pagination={false} size="middle" />
        </Content>
      </Layout>
      <Drawer title="抽屉 1/2 宽验证" placement="right" width="50%" open={open} onClose={() => setOpen(false)}>
        <Typography.Paragraph>确认:抽屉占页面 1/2;分割线低对比不发亮;Tag 有区分度。</Typography.Paragraph>
      </Drawer>
    </Layout>
  )
}
