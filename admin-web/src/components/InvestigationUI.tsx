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
    <label className="[&>span]:text-admin-muted grid min-w-0 gap-[0.42rem] text-[0.76rem] font-medium text-[#d9d4c8] [&>span]:font-mono [&>span]:text-[0.64rem] [&>span]:tracking-[0.04em] [&>span]:uppercase">
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
    <div
      className="[&>button]:border-admin-ink/15 [&>button:not(:disabled)]:hover:border-admin-accent [&>button:not(:disabled)]:hover:text-admin-accent-bright [&>span]:text-admin-muted flex items-center gap-[0.45rem] [&>button]:cursor-pointer [&>button]:rounded-md [&>button]:border [&>button]:bg-white/2 [&>button]:px-[0.7rem] [&>button]:py-[0.48rem] [&>button]:text-[0.72rem] [&>button]:text-[#d9d4c8] [&>button:disabled]:cursor-not-allowed [&>button:disabled]:opacity-35 [&>span]:px-[0.4rem] [&>span]:font-mono [&>span]:text-[0.66rem]"
      aria-label={label}
    >
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
    <div className="flex items-center justify-between gap-4">
      <div>
        <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
          {eyebrow}
        </p>
        <h2
          id={id}
          className="text-admin-ink-strong mt-[0.35rem] mb-0 text-[1.15rem] font-semibold"
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
              ? 'text-[#e0b45e]'
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
      <h3 className="text-admin-ink mt-4 mb-[0.35rem]">{title}</h3>
      <p className="m-0 max-w-85 text-[0.8rem]">{description}</p>
    </div>
  )
}

export function RoomStatus({ value }: { value: string }) {
  const statusClass =
    value === 'in_progress'
      ? 'border-[#2d7a46]/60 bg-[#2d7a46]/17 text-[#72c88d]'
      : value === 'waiting'
        ? 'border-[#c9922b]/45 bg-[#c9922b]/10 text-[#e0b45e]'
        : 'text-[#9c9589]'

  return (
    <span
      className={`border-admin-ink/16 inline-flex items-center rounded-full border px-[0.55rem] py-[0.22rem] font-mono text-[0.56rem] tracking-[0.04em] uppercase ${statusClass}`}
    >
      {formatLabel(value)}
    </span>
  )
}
