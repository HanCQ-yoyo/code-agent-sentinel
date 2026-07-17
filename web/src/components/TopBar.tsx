import { useEffect } from 'react'
import { Layout, Button, Switch, Space, Select, Breadcrumb } from 'antd'
import { ReloadOutlined, HomeOutlined } from '@ant-design/icons'
import { Link, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useTheme } from '../theme'
import { useStore } from '../store'
import { agentMeta } from '../lib/agents'
import { navLabels } from '../lib/nav'
import type { Agent } from '../types'

const { Header } = Layout

interface Props {
  onScan: () => void
  loading: boolean
}

// 末段面包屑文案(含动态 :id 路由)。父段用 navLabels(侧栏文案单一来源)。
function leafLabel(pathname: string, t: (k: string) => string): string | null {
  if (pathname.match(/^\/assets\/[^/]+$/)) return t('topbar.leafAsset')
  if (pathname.match(/^\/history\/[^/]+$/)) return t('topbar.leafScan')
  return null
}

export function TopBar({ onScan, loading }: Props) {
  const { theme, toggle } = useTheme()
  const { t, i18n } = useTranslation()
  const { agents, fetchAgents, language, saveLanguage } = useStore()
  const loc = useLocation()
  const currentAgent = agents?.current

  // 当前一级路由(用于面包屑首段)。navLabels 存 i18n key,需 t() 翻译。
  const root = loc.pathname === '/' ? '/dashboard' : `/${loc.pathname.split('/')[1]}`
  const rootLabel = navLabels[root] ? t(navLabels[root]) : undefined
  const leaf = leafLabel(loc.pathname, t)

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
        <Select
          value={i18n.language?.startsWith('en') ? 'en' : 'zh'}
          onChange={(v) => {
            localStorage.setItem('sentinel.lang', v)
            i18n.changeLanguage(v)
            saveLanguage(v)
          }}
          aria-label={t('topbar.language')}
          style={{ width: 96 }}
          options={[
            { value: 'zh', label: '中文' },
            { value: 'en', label: 'English' },
          ]}
        />
        <Switch
          checked={theme === 'dark'}
          onChange={toggle}
          checkedChildren={t('topbar.dark')}
          unCheckedChildren={t('topbar.light')}
          aria-label={t('topbar.theme')}
        />
        <Button type="primary" icon={<ReloadOutlined />} loading={loading} onClick={onScan}>
          {loading ? t('topbar.scanning') : t('topbar.rescan')}
        </Button>
      </Space>
    </Header>
  )
}
