const BEIJING = 'Asia/Shanghai'

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

export function toBeijingParts(input?: string | Date | null): {
  y: number
  m: number
  d: number
  hh: number
  mm: number
  ss: number
} | null {
  if (!input) return null
  const date = typeof input === 'string' ? new Date(input) : input
  if (Number.isNaN(date.getTime())) return null
  const fmt = new Intl.DateTimeFormat('en-US', {
    timeZone: BEIJING,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  })
  const bag: Record<string, string> = {}
  for (const part of fmt.formatToParts(date)) {
    if (part.type !== 'literal') bag[part.type] = part.value
  }
  return {
    y: Number(bag.year),
    m: Number(bag.month),
    d: Number(bag.day),
    hh: Number(bag.hour),
    mm: Number(bag.minute),
    ss: Number(bag.second),
  }
}

export function formatBeijing(input?: string | Date | null): string {
  const p = toBeijingParts(input)
  if (!p) return '—'
  return `${p.y}-${pad(p.m)}-${pad(p.d)} ${pad(p.hh)}:${pad(p.mm)}:${pad(p.ss)}`
}

export function formatBeijingDate(input?: string | Date | null): string {
  const p = toBeijingParts(input)
  if (!p) return '—'
  return `${p.y}-${pad(p.m)}-${pad(p.d)}`
}

export function beijingTodayISO(): string {
  const p = toBeijingParts(new Date())!
  return `${p.y}-${pad(p.m)}-${pad(p.d)}`
}

export function parseISODate(value?: string | null): Date | null {
  if (!value) return null
  const d = new Date(value.includes('T') ? value : `${value}T00:00:00+08:00`)
  return Number.isNaN(d.getTime()) ? null : d
}

export function dateKey(input: string | Date): string {
  const p = toBeijingParts(input)
  if (!p) return ''
  return `${p.y}-${pad(p.m)}-${pad(p.d)}`
}

export function addDays(isoDate: string, days: number): string {
  const d = parseISODate(isoDate)
  if (!d) return isoDate
  d.setTime(d.getTime() + days * 86400000)
  return formatBeijingDate(d)
}

export function daysBetween(a: string, b: string): number {
  const da = parseISODate(a)
  const db = parseISODate(b)
  if (!da || !db) return 0
  return Math.round((db.getTime() - da.getTime()) / 86400000)
}
