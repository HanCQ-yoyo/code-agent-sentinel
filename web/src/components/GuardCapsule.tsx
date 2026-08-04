import { Button, Popover, Space, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { useEffect } from 'react'

export function GuardCapsule() {
  const { t } = useTranslation()
  const { guardConfig, guardToggling, fetchGuardConfig, toggleGuard } = useStore()

  // 初始加载(如果未加载)
  useEffect(() => {
    if (!guardConfig) fetchGuardConfig()
  }, [guardConfig, fetchGuardConfig])

  const enabled = guardConfig?.enabled ?? false
  const loading = guardToggling

  // 开启态样式:绿色调
  const onStyle: React.CSSProperties = {
    color: 'var(--color-success)',
    borderColor: 'var(--color-success)',
    background: 'transparent',
    fontWeight: 500,
    fontSize: 'var(--fs-sm)',
    padding: '0 12px',
    height: 32,
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  }

  // 关闭态样式:灰色调
  const offStyle: React.CSSProperties = {
    color: 'var(--color-text-muted)',
    borderColor: 'var(--color-rule)',
    background: 'transparent',
    fontWeight: 500,
    fontSize: 'var(--fs-sm)',
    padding: '0 12px',
    height: 32,
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  }

  const style = enabled ? onStyle : offStyle

  // 实心/空心圆点
  const dot = enabled ? (
    <span style={{ color: 'var(--color-success)', fontSize: 10, lineHeight: 1 }}>●</span>
  ) : (
    <span style={{ color: 'var(--color-text-muted)', fontSize: 10, lineHeight: 1 }}>○</span>
  )

  const label = loading
    ? t('guardCapsule.capsuleToggling')
    : enabled
      ? t('guardCapsule.capsuleOn')
      : t('guardCapsule.capsuleOff')

  const handleClick = () => {
    if (loading) return
    // 关闭→开启:直接切换(无确认)
    toggleGuard()
  }

  // 开启态用 Popover 包装(需要确认关闭)
  if (enabled && !loading) {
    return (
      <Popover
        trigger="click"
        placement="bottomRight"
        content={
          <div style={{ maxWidth: 260 }}>
            <Typography.Text style={{ fontSize: 'var(--fs-sm)' }}>
              {t('guardCapsule.confirmOff')}
            </Typography.Text>
            <div style={{ marginTop: 12, textAlign: 'right' }}>
              <Space>
                <Button size="small">
                  {t('common.cancel')}
                </Button>
                <Button size="small" danger onClick={() => toggleGuard()}>
                  {t('guardCapsule.confirmOk')}
                </Button>
              </Space>
            </div>
          </div>
        }
      >
        <Button style={style}>
          {dot}
          {label}
        </Button>
      </Popover>
    )
  }

  return (
    <Button style={style} onClick={handleClick} disabled={loading} loading={loading}>
      {!loading && dot}
      {label}
    </Button>
  )
}
