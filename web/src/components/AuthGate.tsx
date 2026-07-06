import { getAuthToken } from '../api/client'
import { useStore } from '../store'

export function AuthGate({ children }: { children: React.ReactNode }) {
  const authError = useStore(s => s.authError)
  const hasToken = getAuthToken() !== ''
  if (!hasToken || authError) {
    return (
      <div className="min-h-screen flex items-center justify-center p-6">
        <div className="max-w-md text-center space-y-3">
          <h1 className="text-2xl font-semibold">需要访问 token</h1>
          <p className="text-slate-400">
            Sentinel 通过 token 鉴权。请用启动时控制台打印的本地访问 URL(含 <code className="px-1 py-0.5 rounded bg-bg-border font-mono text-sm">#token=</code> 片段)打开。
          </p>
          <p className="text-sm text-slate-500">
            token 仅经 URL fragment 传递,不进 server log / Referer。重新扫描报 401 时也回到这里 —— 重新用带 token 的 URL 访问即可。
          </p>
        </div>
      </div>
    )
  }
  return <>{children}</>
}
