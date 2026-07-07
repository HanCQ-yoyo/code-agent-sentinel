import type { ReactNode } from 'react'
import { Result, Input, Typography } from 'antd'
import { useStore } from '../store'
import { getAuthToken } from '../api/client'

export function AuthGate({ children }: { children: ReactNode }) {
  const authError = useStore((s) => s.authError)
  const hasToken = getAuthToken() !== ''
  if (!hasToken || authError) {
    return (
      <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 24 }}>
        <Result
          status="warning"
          title="需要访问 token"
          subTitle={
            <Typography.Paragraph>
              通过 URL fragment 传递 token,例如访问:
              <Typography.Text code>http://127.0.0.1:&lt;port&gt;/#token=&lt;your-token&gt;</Typography.Text>
            </Typography.Paragraph>
          }
          extra={<Input placeholder="token 经 URL fragment 传递,无需手填" disabled />}
        />
      </div>
    )
  }
  return <>{children}</>
}
