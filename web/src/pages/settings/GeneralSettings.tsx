import { useEffect } from 'react'
import { Card, Select, Table, Switch, Typography, Tooltip } from 'antd'
import { InfoCircleOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useStore } from '../../store'
import { useTheme } from '../../theme'
import type { Agent } from '../../types'
import { AgentIcon } from '../../components/AgentIcon'

export default function GeneralSettings() {
  const { t, i18n } = useTranslation()
  const { agents, fetchAgents, saveAgentScanEnabled, language, saveLanguage } = useStore()
  const { theme, toggle } = useTheme()

  useEffect(() => { if (!agents) fetchAgents() }, [agents, fetchAgents])

  const cmd = (t('settings.agentsReadonlyDesc').match(/`([^`]+)`/)?.[1]) ?? 'sentinel setup'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 个性化配置: 主题 + 语言,标题在左控件在右 */}
      <Card title={t('settings.personalization', { defaultValue: '个性化配置' })} size="small">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <span style={{ fontSize: 'var(--fs-sm)' }}>{t('topbar.theme')}</span>
            <Switch
              size="small"
              checked={theme === 'dark'}
              onChange={toggle}
              checkedChildren={t('topbar.dark')}
              unCheckedChildren={t('topbar.light')}
              aria-label={t('topbar.theme')}
            />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <span style={{ fontSize: 'var(--fs-sm)' }}>{t('topbar.language')}</span>
            <Select
              value={i18n.language === 'zh' ? 'zh' : 'en'}
              onChange={(v) => {
                localStorage.setItem('sentinel.lang', v)
                i18n.changeLanguage(v)
                saveLanguage(v)
              }}
              aria-label={t('topbar.language')}
              style={{ width: 120 }}
              size="small"
              options={[
                { value: 'zh', label: '中文' },
                { value: 'en', label: 'English' },
              ]}
            />
          </div>
        </div>
      </Card>

      {/* Code Agents(原 SettingsAgents 内容) */}
      <Card title={t('settings.agentsTab')} size="small">
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)', marginBottom: 'var(--space-md)', color: 'var(--color-muted)', fontSize: 'var(--fs-sm)' }}>
          <InfoCircleOutlined style={{ color: 'var(--color-dim)' }} />
          <span>{t('settings.agentsReadonlyHint')}</span>
          <span style={{ color: 'var(--color-dim)' }}>·</span>
          <span>{t('settings.agentsReadonlyDesc').replace(/`([^`]+)`/g, '')}</span>
          <Tooltip title={t('common.copy')}>
            <Typography.Text code copyable style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)' }}>{cmd}</Typography.Text>
          </Tooltip>
        </div>
        <Table
          size="small"
          dataSource={agents?.agents ?? []}
          rowKey="id"
          pagination={false}
          columns={[
            { title: t('settings.agentName'), dataIndex: 'name', render: (name: string, r: Agent) => (
              <span style={{ whiteSpace: 'nowrap' }}><AgentIcon id={r.id} /> {name}</span>
            ) },
            { title: 'ID', dataIndex: 'id' },
            { title: t('settings.rootDir'), dataIndex: 'root_dir' },
            { title: t('settings.claudeJson'), dataIndex: 'claude_json' },
            { title: t('settings.scanSwitch'), dataIndex: 'scan_enabled', width: 100,
              render: (enabled: boolean, record: Agent) => (
                <Switch checked={enabled} onChange={(checked) => saveAgentScanEnabled(record.id, checked)} />
              ),
            },
          ]}
        />
      </Card>
    </div>
  )
}
