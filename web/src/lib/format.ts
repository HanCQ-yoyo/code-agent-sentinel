// formatDateTime:ISO 字符串 → 本地 YYYY-MM-DD HH:mm:ss(各部分补零)。
// 空输入返回 '--'。用于历史列表/详情的时间列。
export function formatDateTime(iso: string): string {
  if (!iso) return '--'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '--'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

// formatDateTimeShort:列表用的紧凑时间(MM-DD HH:mm,省年份与秒),缓解历史列表时间列偏宽。
// 详情页仍用 formatDateTime(完整格式)。
export function formatDateTimeShort(iso: string): string {
  if (!iso) return '--'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '--'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
