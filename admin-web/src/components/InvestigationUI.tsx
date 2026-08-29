import type { ReactNode } from 'react'
import { formatLabel } from './formatters'

export function FilterField({
  label,
  children,
}: {
  label: string
  children: ReactNode
}) {
  return (
    <label className="game-field">
      <span>{label}</span>
      {children}
    </label>
  )
}

export function Pagination({
  offset,
  pageSize,
  itemCount,
  loading,
  onOffsetChange,
  label,
}: {
  offset: number
  pageSize: number
  itemCount: number
  loading: boolean
  onOffsetChange: (offset: number) => void
  label: string
}) {
  return (
    <div className="pagination" aria-label={label}>
      <button
        type="button"
        onClick={() => onOffsetChange(Math.max(0, offset - pageSize))}
        disabled={offset === 0 || loading}
      >
        Previous
      </button>
      <span>Page {Math.floor(offset / pageSize) + 1}</span>
      <button
        type="button"
        onClick={() => onOffsetChange(offset + pageSize)}
        disabled={itemCount < pageSize || loading}
      >
        Next
      </button>
    </div>
  )
}

export function SectionHeading({
  eyebrow,
  title,
  id,
  meta,
}: {
  eyebrow: string
  title: string
  id: string
  meta?: ReactNode
}) {
  return (
    <div className="section-heading">
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h2 id={id}>{title}</h2>
      </div>
      {meta !== undefined ? <span>{meta}</span> : null}
    </div>
  )
}

export function SummaryItem({
  label,
  value,
  tone,
}: {
  label: string
  value: ReactNode
  tone?: 'healthy' | 'warning'
}) {
  return (
    <div>
      <dt>{label}</dt>
      <dd className={tone ? `summary-${tone}` : ''}>{value}</dd>
    </div>
  )
}

export function EmptyState({
  mark,
  title,
  description,
  className = 'empty-state',
}: {
  mark: string
  title: string
  description: string
  className?: string
}) {
  return (
    <div className={className}>
      <span className={className === 'empty-state' ? 'empty-mark' : undefined}>
        {mark}
      </span>
      <h3>{title}</h3>
      <p>{description}</p>
    </div>
  )
}

export function RoomStatus({ value }: { value: string }) {
  return (
    <span className={`room-status room-status-${value}`}>
      {formatLabel(value)}
    </span>
  )
}
