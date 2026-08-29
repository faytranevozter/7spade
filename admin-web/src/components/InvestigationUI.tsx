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
    <label className="text-admin-ink-soft gap-admin-badge-x text-admin-form grid min-w-0 font-medium">
      <span className="text-admin-muted text-admin-filter font-mono tracking-[0.04em] uppercase">
        {label}
      </span>
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
    <div className="gap-admin-6 flex items-center" aria-label={label}>
      <button
        type="button"
        onClick={() => onOffsetChange(Math.max(0, offset - pageSize))}
        disabled={offset === 0 || loading}
        className="border-admin-ink/15 enabled:hover:border-admin-accent enabled:hover:text-admin-accent-bright px-admin-10 text-admin-ink-soft text-admin-button cursor-pointer rounded-md border bg-white/2 py-[0.48rem] disabled:cursor-not-allowed disabled:opacity-35"
      >
        Previous
      </button>
      <span className="text-admin-muted px-admin-5 font-mono text-[0.66rem]">
        Page {Math.floor(offset / pageSize) + 1}
      </span>
      <button
        type="button"
        onClick={() => onOffsetChange(offset + pageSize)}
        disabled={itemCount < pageSize || loading}
        className="border-admin-ink/15 enabled:hover:border-admin-accent enabled:hover:text-admin-accent-bright px-admin-10 text-admin-ink-soft text-admin-button cursor-pointer rounded-md border bg-white/2 py-[0.48rem] disabled:cursor-not-allowed disabled:opacity-35"
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
    <div className="flex items-center justify-between gap-4">
      <div>
        <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
          {eyebrow}
        </p>
        <h2
          id={id}
          className="text-admin-ink-strong mt-admin-4 text-admin-section mb-0 font-semibold"
        >
          {title}
        </h2>
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
      <dd
        className={
          tone === 'healthy'
            ? 'text-admin-success'
            : tone === 'warning'
              ? 'text-admin-warning'
              : undefined
        }
      >
        {value}
      </dd>
    </div>
  )
}

export function EmptyState({
  mark,
  title,
  description,
  className = '',
  markClassName = '',
}: {
  mark: string
  title: string
  description: string
  className?: string
  markClassName?: string
}) {
  return (
    <div
      className={`border-admin-ink/14 text-admin-muted grid min-h-75 place-items-center content-center rounded-xl border border-dashed text-center ${className}`.trim()}
    >
      <span
        className={`border-admin-accent/40 text-admin-accent grid h-16 w-13.5 place-items-center rounded-lg border font-mono ${markClassName}`.trim()}
      >
        {mark}
      </span>
      <h3 className="text-admin-ink mb-admin-4 mt-4">{title}</h3>
      <p className="m-0 max-w-85 text-[0.8rem]">{description}</p>
    </div>
  )
}

export function RoomStatus({ value }: { value: string }) {
  return (
    <span
      className={`border-admin-border-subtle py-admin-chip-y text-admin-tiny inline-flex items-center rounded-full border px-[0.55rem] font-mono tracking-[0.04em] uppercase ${value === 'in_progress' ? 'border-admin-success-border bg-admin-success-bg text-admin-success' : value === 'waiting' ? 'border-admin-accent-border-subtle bg-admin-accent-soft text-admin-warning' : 'text-admin-muted'}`}
    >
      {formatLabel(value)}
    </span>
  )
}
