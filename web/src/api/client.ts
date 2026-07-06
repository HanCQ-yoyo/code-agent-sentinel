// token 首屏捕获一次,持久化到 sessionStorage。
// 修复:旧行为每次 req() 都从 window.location.hash 现读,React Router pushState
// (点 NavLink 导航)会丢 #token fragment → 导航后送空 token → 401(问题 3)。
// sessionStorage 跨刷新保留,关闭标签页清除(不进 localStorage 避免持久泄露)。
const TOKEN_KEY = 'sentinel_token'

function captureToken(): string {
  // 1. URL hash 优先(刚从启动 URL 进来)
  const m = window.location.hash.match(/token=([A-Za-z0-9_-]+)/)
  if (m) {
    const t = m[1]
    sessionStorage.setItem(TOKEN_KEY, t)
    return t
  }
  // 2. 已持久化的
  return sessionStorage.getItem(TOKEN_KEY) ?? ''
}

let cachedToken: string | null = null
export function getAuthToken(): string {
  if (cachedToken === null) cachedToken = captureToken()
  return cachedToken
}

export function clearAuthToken(): void {
  cachedToken = ''
  sessionStorage.removeItem(TOKEN_KEY)
}

export class AuthError extends Error {}

async function req(path: string, method = 'GET'): Promise<Response> {
  const r = await fetch(path, { method, headers: { Authorization: `Bearer ${getAuthToken()}` } })
  if (r.status === 401) {
    // token 失效:清缓存,触发 AuthGate 重新提示
    clearAuthToken()
    throw new AuthError('missing or invalid token')
  }
  if (!r.ok) throw new Error(`${r.status} ${await r.text()}`)
  return r
}

export const apiGet = <T>(p: string) => req(p).then(r => r.json() as Promise<T>)
export const apiPost = <T>(p: string) => req(p, 'POST').then(r => r.json() as Promise<T>)
