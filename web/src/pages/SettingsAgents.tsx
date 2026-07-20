import { useEffect } from 'react'
import { Table, Alert } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'

// Settings 页「Code Agents」只读 tab:展示已启用 agent(id/name/路径)。
// 修改走 `sentinel setup` + 重启,前端不提供编辑入口。
export function SettingsAgents() {
  const { t } = useTranslation()
  const { agents, fetchAgents } = useStore()
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
          { title: 'ID', dataIndex: 'id' },
          { title: t('settings.rootDir'), dataIndex: 'root_dir' },
          { title: t('settings.claudeJson'), dataIndex: 'claude_json' },
        ]}
      />
    </div>
  )
}
