import { useEffect, useState } from 'react'
import { Card, Input, Button, List, Space, Popconfirm, Typography, message, Empty } from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'

// Settings 页「放行清单」tab(Stage R3 Task 12)。
//
// 后端 PUT /api/guard/allowlist 要求 {allowlist: [...]} 全量替换(见 handlers_allowlist.go):
// 顶层键缺失 → 400;空数组 = 清空。本组件 fetch 全量 → 本地 list 增删 → 保存时整体 PUT list。
//
// 放行语义:精确命令匹配,不支持通配。命中的命令在管线 ⑦ 被放行(即便命中规则也允许执行)。
// 仅当 guard.allowlist_enabled=true 时管线 ⑦ 才生效(开关在「拦截配置」tab)。
export function SettingsAllowlist() {
  const { t } = useTranslation()
  const { allowlist, fetchAllowlist, saveAllowlist } = useStore()
  const [input, setInput] = useState('')
  const [list, setList] = useState<string[]>([])
  const [saving, setSaving] = useState(false)

  useEffect(() => { fetchAllowlist() }, [fetchAllowlist])
  // store.allowlist → 本地 list(只在 store 变化时同步,避免覆盖用户正在编辑的值)。
  useEffect(() => { setList(allowlist) }, [allowlist])

  const add = () => {
    const cmd = input.trim()
    if (!cmd) return
    if (list.includes(cmd)) {
      message.warning(t('guard.allowlistDup'))
      return
    }
    setList([...list, cmd])
    setInput('')
  }
  const remove = (cmd: string) => setList(list.filter((c) => c !== cmd))

  const onSave = async () => {
    setSaving(true)
    const ok = await saveAllowlist(list)
    setSaving(false)
    if (ok) message.success(t('guard.saved'))
  }

  return (
    <Card
      title={t('guard.allowlistTitle')}
      extra={<Button type="primary" loading={saving} onClick={onSave}>{t('common.save')}</Button>}
    >
      <Typography.Text type="secondary">{t('guard.allowlistHint')}</Typography.Text>
      <Space.Compact style={{ width: '100%', margin: '12px 0' }}>
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onPressEnter={add}
          placeholder={t('guard.allowlistPlaceholder')}
        />
        <Button icon={<PlusOutlined />} onClick={add}>{t('guard.add')}</Button>
      </Space.Compact>
      {list.length === 0 ? (
        <Empty description={t('guard.allowlistEmpty')} />
      ) : (
        <List
          bordered
          dataSource={list}
          renderItem={(cmd) => (
            <List.Item
              actions={[
                <Popconfirm
                  key="del"
                  title={t('common.confirmDelete')}
                  okText={t('common.delete')}
                  okButtonProps={{ danger: true }}
                  cancelText={t('common.cancel')}
                  onConfirm={() => remove(cmd)}
                >
                  <Button danger icon={<DeleteOutlined />} />
                </Popconfirm>,
              ]}
            >
              <Typography.Text code>{cmd}</Typography.Text>
            </List.Item>
          )}
        />
      )}
    </Card>
  )
}
