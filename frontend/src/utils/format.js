// 金额（分）与日期格式化工具

// 123456 -> "¥1,234.56"；负数 -> "-¥12.00"
export function formatCents(cents) {
  const n = Number(cents) || 0
  const sign = n < 0 ? '-' : ''
  const abs = Math.abs(n)
  const yuan = (abs / 100).toFixed(2)
  const [int, dec] = yuan.split('.')
  const intFmt = int.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return `${sign}¥${intFmt}.${dec}`
}

// 分 -> 元字符串（用于编辑输入框）
export function centsToYuan(cents) {
  const n = Number(cents) || 0
  return (n / 100).toFixed(2)
}

// 元字符串 -> 分（整数）
export function yuanToCents(yuan) {
  const n = Number(yuan)
  if (!Number.isFinite(n)) return 0
  return Math.round(n * 100)
}

// 今天 YYYY-MM-DD
export function today() {
  const d = new Date()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

export function fmtDate(s) {
  return s || ''
}
