export function formatLabel(value?: string) {
  if (!value) return 'Unknown'
  return value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
}

export function formatDateTime(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

export function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, { timeStyle: 'medium' }).format(new Date(value))
}

export function toLocalDateTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

export function shortID(value: string, leading = 8) {
  return value.length > leading + 7 ? `${value.slice(0, leading)}...${value.slice(-4)}` : value
}
