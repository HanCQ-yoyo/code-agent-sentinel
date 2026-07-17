import type { ReactNode } from 'react'
import { Result, Input, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import { useStore } from '../store'
import { getAuthToken } from '../api/client'

export function AuthGate({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const authError = useStore((s) => s.authError)
  const hasToken = getAuthToken() !== ''
  if (!hasToken || authError) {
    return (
      // role="alert" 放外层 div:antd Result 的 ResultProps 不含 role,且实现不 spread rest 到根 div,
      // 放 Result 上既过不了 TS 也不到 DOM。外层 div 语义上等价(screen-reader 仍宣告整个区域为 alert)。
      <div role="alert" style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 24 }}>
        <Result
          status="warning"
          title={t('auth.title')}
          subTitle={
            <Typography.Paragraph>
              {t('auth.desc')}
              <Typography.Text code>http://127.0.0.1:&lt;port&gt;/#token=&lt;your-token&gt;</Typography.Text>
            </Typography.Paragraph>
          }
          extra={<Input placeholder={t('auth.placeholder')} disabled />}
        />
      </div>
    )
  }
  return <>{children}</>
}
