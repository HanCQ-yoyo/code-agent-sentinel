function token(): string {
  const m = window.location.hash.match(/token=([A-Za-z0-9_-]+)/)
  return m ? m[1] : ''
}
async function req(path: string, method = 'GET'): Promise<Response> {
  const r = await fetch(path, { method, headers: { Authorization: `Bearer ${token()}` } })
  if (!r.ok) throw new Error(`${r.status} ${await r.text()}`)
  return r
}
export const apiGet = <T>(p: string) => req(p).then(r => r.json() as Promise<T>)
export const apiPost = <T>(p: string) => req(p, 'POST').then(r => r.json() as Promise<T>)
