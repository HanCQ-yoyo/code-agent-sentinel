// formatDateTime:ISO 字符串 → 本地 YYYY-MM-DD HH:mm:ss(各部分补零)。
// 空输入返回 '--'。用于历史列表/详情的时间列。
export function formatDateTime(iso: string): string {
  if (!iso) return '--'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '--'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

// formatDateTimeShort:列表用的时间(YYYY-MM-DD HH:mm,带年份省秒)。
// 详情页仍用 formatDateTime(带秒)。用户反馈列表缺年份,此处补回年份。
export function formatDateTimeShort(iso: string): string {
  if (!iso) return '--'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '--'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
