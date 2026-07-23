import { useEffect } from 'react'
import { Table, Alert, Switch } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import type { Agent } from '../types'

// Settings 页「Code Agents」tab:展示已注册 agent。
// agent 的*配置*(id/name/路径/Enabled 加载标志)只读——修改走 `sentinel setup` + 重启。
// 但 per-agent 的*扫描开关*(ScanEnabled)是运行期覆盖(PUT /api/agents/:id),前端可在此切换:
// 关闭后该 agent 不参与定时扫描(定时任务暂停);手动重扫描仍可强制指定。
export function SettingsAgents() {
  const { t } = useTranslation()
  const { agents, fetchAgents, saveAgentScanEnabled } = useStore()
  // 复用 TopBar 的守卫模式:agents 已加载(TopBar 早一步拉过)则不重复请求。
  useEffect(() => { if (!agents) fetchAgents() }, [agents, fetchAgents])
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Alert type="info" showIcon message={t('settings.agentsReadonlyHint')} description={t('settings.agentsReadonlyDesc')} />
      <Table
        size="small"
        dataSource={agents?.agents ?? []}
        rowKey="id"
        pagination={false}
        columns={[
          { title: t('settings.agentName'), dataIndex: 'name' },
          {
            title: t('settings.scanSwitch'),
            dataIndex: 'scan_enabled',
            width: 100,
            render: (enabled: boolean, record: Agent) => (
              <Switch checked={enabled} onChange={(checked) => saveAgentScanEnabled(record.id, checked)} />
            ),
          },
          { title: 'ID', dataIndex: 'id' },
          { title: t('settings.rootDir'), dataIndex: 'root_dir' },
          { title: t('settings.claudeJson'), dataIndex: 'claude_json' },
        ]}
      />
    </div>
  )
}
