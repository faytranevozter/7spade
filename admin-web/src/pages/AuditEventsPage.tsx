import { type FormEvent, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import {
  exportAuditEvents,
  getAuditEvents,
  type AuditEvent,
  type AuditFilters,
} from '../api/audit'
import { Notice } from '../components/Feedback'
import { useAuth } from '../hooks/useAuth'

const pageSize = 50
const filterKeys = [
  'id',
  'actor_id',
  'action',
  'resource_type',
  'resource_id',
  'outcome',
  'from',
  'to',
] as const

type FilterKey = (typeof filterKeys)[number]
type FilterForm = Record<FilterKey, string>

const emptyFilters: FilterForm = {
  id: '',
  actor_id: '',
  action: '',
  resource_type: '',
  resource_id: '',
  outcome: '',
  from: '',
  to: '',
}

function filtersFromParams(params: URLSearchParams): FilterForm {
  return Object.fromEntries(
    filterKeys.map((key) => [key, params.get(key) ?? '']),
  ) as FilterForm
}

function apiFilters(filters: FilterForm): AuditFilters {
  return {
    ...filters,
    from: filters.from ? new Date(filters.from).toISOString() : undefined,
    to: filters.to ? new Date(filters.to).toISOString() : undefined,
  }
}

function formatTimestamp(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(new Date(value))
}

function outcomeClass(outcome: string) {
  if (outcome === 'success')
    return 'border-admin-success-border bg-admin-success-bg text-admin-success'
  return 'border-admin-danger-border bg-admin-danger-bg text-admin-danger'
}

export function AuditEventsPage() {
  const [searchParams] = useSearchParams()
  return <AuditEventsContent key={searchParams.toString()} />
}

function AuditEventsContent() {
  const { admin, token } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()
  const appliedFilters = filtersFromParams(searchParams)
  const [form, setForm] = useState(appliedFilters)
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [exporting, setExporting] = useState(false)
  const offset = Math.max(0, Number(searchParams.get('offset')) || 0)
  const queryKey = searchParams.toString()
  const canExport = admin?.permissions.includes('audit.export') ?? false
  const exportRangeValid = (() => {
    if (!appliedFilters.from || !appliedFilters.to) return false
    const from = new Date(appliedFilters.from).getTime()
    const to = new Date(appliedFilters.to).getTime()
    return to > from && to - from <= 31 * 24 * 60 * 60 * 1000
  })()

  useEffect(() => {
    if (!token) return
    let active = true
    const requestedFilters = filtersFromParams(new URLSearchParams(queryKey))
    getAuditEvents(token, apiFilters(requestedFilters), pageSize, offset)
      .then((page) => {
        if (active) setEvents(page.events)
      })
      .catch((cause: unknown) => {
        if (active)
          setError(
            cause instanceof Error
              ? cause.message
              : 'Failed to load audit events',
          )
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [token, queryKey, offset])

  if (!token) return null

  const updateForm = (key: FilterKey, value: string) =>
    setForm((current) => ({ ...current, [key]: value }))

  const applyFilters = (event: FormEvent) => {
    event.preventDefault()
    const params = new URLSearchParams()
    for (const key of filterKeys) if (form[key]) params.set(key, form[key])
    setSearchParams(params)
  }

  const changePage = (nextOffset: number) => {
    const params = new URLSearchParams(searchParams)
    if (nextOffset > 0) params.set('offset', String(nextOffset))
    else params.delete('offset')
    setSearchParams(params)
  }

  const downloadExport = async () => {
    if (!exportRangeValid) return
    setExporting(true)
    setError('')
    try {
      const blob = await exportAuditEvents(token, apiFilters(appliedFilters))
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = 'audit-events.csv'
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : 'Failed to export audit events',
      )
    } finally {
      setExporting(false)
    }
  }

  return (
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="audit-heading"
    >
      <header className="border-admin-border border-b pb-8">
        <p className="text-admin-accent text-admin-label m-0 font-mono tracking-wider uppercase">
          Security and operations
        </p>
        <h1
          id="audit-heading"
          className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-hero leading-[0.95] font-medium tracking-[-0.06em]"
        >
          Audit log
        </h1>
        <p className="text-admin-muted m-0 max-w-170 leading-[1.65]">
          Trace administrator activity, affected resources, and request
          outcomes.
        </p>
      </header>

      <form
        onSubmit={applyFilters}
        className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent mt-6 grid gap-4 border p-5"
      >
        <div className="grid grid-cols-3 gap-4 max-[1000px]:grid-cols-2 max-[620px]:grid-cols-1">
          <FilterInput
            label="Action"
            value={form.action}
            onChange={(value) => updateForm('action', value)}
            placeholder="admin.status.update"
          />
          <FilterInput
            label="Actor ID"
            value={form.actor_id}
            onChange={(value) => updateForm('actor_id', value)}
            placeholder="Administrator UUID"
          />
          <FilterInput
            label="Event ID"
            value={form.id}
            onChange={(value) => updateForm('id', value)}
            placeholder="Audit event UUID"
          />
          <FilterInput
            label="Resource type"
            value={form.resource_type}
            onChange={(value) => updateForm('resource_type', value)}
            placeholder="admin_user"
          />
          <FilterInput
            label="Resource ID"
            value={form.resource_id}
            onChange={(value) => updateForm('resource_id', value)}
            placeholder="Affected resource"
          />
          <label className="gap-admin-5 text-admin-field text-admin-muted grid">
            <span className="text-admin-label font-mono tracking-wider uppercase">
              Outcome
            </span>
            <select
              className={inputClass}
              value={form.outcome}
              onChange={(event) => updateForm('outcome', event.target.value)}
            >
              <option value="">All outcomes</option>
              <option value="success">Success</option>
              <option value="rejected">Rejected</option>
              <option value="failed">Failed</option>
            </select>
          </label>
          <FilterInput
            type="datetime-local"
            label="From"
            value={form.from}
            onChange={(value) => updateForm('from', value)}
          />
          <FilterInput
            type="datetime-local"
            label="To"
            value={form.to}
            onChange={(value) => updateForm('to', value)}
          />
        </div>
        <div className="border-admin-border-divider flex flex-wrap items-center gap-3 border-t pt-4">
          <button type="submit" className={primaryButtonClass}>
            Apply filters
          </button>
          <button
            type="button"
            className={secondaryButtonClass}
            onClick={() => {
              setForm(emptyFilters)
              setSearchParams({})
            }}
          >
            Reset
          </button>
          {canExport ? (
            <button
              type="button"
              className={`${secondaryButtonClass} ml-auto`}
              disabled={!exportRangeValid || exporting}
              onClick={downloadExport}
              title={
                exportRangeValid
                  ? ''
                  : 'Apply a date range of no more than 31 days'
              }
            >
              {exporting ? 'Exporting...' : 'Export CSV'}
            </button>
          ) : null}
        </div>
      </form>

      {error ? <Notice variant="error">{error}</Notice> : null}
      {loading ? (
        <p className="text-admin-muted mt-6" aria-live="polite">
          Loading audit events...
        </p>
      ) : null}
      {!loading && !error && events.length === 0 ? (
        <div className="text-admin-muted rounded-admin-panel border-admin-border-input mt-6 grid min-h-48 place-items-center border border-dashed p-8 text-center">
          No audit events match these filters.
        </div>
      ) : null}
      {!loading && events.length > 0 ? (
        <div className="border-admin-border-subtle rounded-admin-panel relative mt-6 overflow-x-auto border">
          <table className="w-full min-w-230 border-collapse text-left">
            <thead className="bg-admin-surface-raised text-admin-label text-admin-muted-subtle font-mono tracking-wider uppercase">
              <tr>
                <th className={headerCellClass}>Occurred</th>
                <th className={headerCellClass}>Action</th>
                <th className={headerCellClass}>Actor</th>
                <th className={headerCellClass}>Resource</th>
                <th className={headerCellClass}>Outcome</th>
                <th className={headerCellClass}>
                  <span className="sr-only">Details</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {events.map((event) => (
                <tr
                  key={event.id}
                  className="border-admin-border-divider border-t"
                >
                  <td className={`${cellClass} whitespace-nowrap`}>
                    {formatTimestamp(event.occurred_at)}
                  </td>
                  <td className={`${cellClass} text-admin-ink-soft font-mono`}>
                    {event.action}
                  </td>
                  <td
                    className={`${cellClass} max-w-48 truncate font-mono`}
                    title={event.actor_id}
                  >
                    {event.actor_id || 'System'}
                  </td>
                  <td className={cellClass}>
                    {event.resource_type || '—'}
                    {event.resource_id ? (
                      <span
                        className="text-admin-muted-subtle block max-w-48 truncate font-mono"
                        title={event.resource_id}
                      >
                        {event.resource_id}
                      </span>
                    ) : null}
                  </td>
                  <td className={cellClass}>
                    <span
                      className={`text-admin-xs rounded-full border px-2 py-1 font-mono uppercase ${outcomeClass(event.outcome)}`}
                    >
                      {event.outcome}
                    </span>
                  </td>
                  <td className={cellClass}>
                    <Link
                      className="text-admin-accent whitespace-nowrap"
                      to={`/audit-events/${event.id}?returnTo=${encodeURIComponent(`/audit-events?${searchParams}`)}`}
                    >
                      View details
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      {!loading && !error && (offset > 0 || events.length === pageSize) ? (
        <nav
          className="border-admin-border-divider mt-6 flex items-center justify-between gap-4 border-t pt-4"
          aria-label="Audit log pages"
        >
          <span className="text-admin-muted text-admin-field">
            Showing {offset + 1}–{offset + events.length}
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              className={secondaryButtonClass}
              disabled={offset === 0}
              onClick={() => changePage(Math.max(0, offset - pageSize))}
            >
              Previous
            </button>
            <button
              type="button"
              className={secondaryButtonClass}
              disabled={events.length < pageSize}
              onClick={() => changePage(offset + pageSize)}
            >
              Next
            </button>
          </div>
        </nav>
      ) : null}
    </section>
  )
}

function FilterInput({
  label,
  value,
  onChange,
  placeholder,
  type = 'text',
}: {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  type?: string
}) {
  return (
    <label className="gap-admin-5 text-admin-field text-admin-muted grid">
      <span className="text-admin-label font-mono tracking-wider uppercase">
        {label}
      </span>
      <input
        className={inputClass}
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  )
}

const inputClass =
  'border-admin-border-input bg-admin-input text-admin-ink rounded-admin-input focus:border-admin-accent-border focus:outline-none border px-3 py-2.5'
const primaryButtonClass =
  'bg-admin-accent border-admin-accent-border text-admin-button-ink rounded-admin-input cursor-pointer border px-4 py-2.5 font-bold disabled:cursor-not-allowed disabled:opacity-45'
const secondaryButtonClass =
  'border-admin-border-input text-admin-ink-soft hover:border-admin-accent-border hover:text-admin-accent rounded-admin-input cursor-pointer border bg-transparent px-4 py-2.5 font-semibold disabled:cursor-not-allowed disabled:opacity-45'
const headerCellClass = 'px-4 py-3'
const cellClass = 'text-admin-muted text-admin-field px-4 py-4 align-top'
