import { useEffect, useState } from 'react'
import { Link, useSearchParams, useParams } from 'react-router'
import { getAuditEvent, type AuditEvent } from '../api/audit'
import { Notice } from '../components/Feedback'
import { useAuth } from '../hooks/useAuth'

export function AuditEventPage() {
  const { id = '' } = useParams()
  const [searchParams] = useSearchParams()
  const { token } = useAuth()
  const [event, setEvent] = useState<AuditEvent | null>(null)
  const [loading, setLoading] = useState(true)
  const [message, setMessage] = useState('')

  useEffect(() => {
    if (!token || !id) return
    let active = true
    getAuditEvent(token, id)
      .then((result) => {
        if (!active) return
        setEvent(result ?? null)
        setMessage(result ? '' : 'Audit event not found.')
        setLoading(false)
      })
      .catch((error: unknown) => {
        if (active) {
          setMessage(
            error instanceof Error
              ? error.message
              : 'Failed to load audit event',
          )
          setLoading(false)
        }
      })
    return () => {
      active = false
    }
  }, [id, token])

  const requestedReturn = searchParams.get('returnTo') ?? ''
  const returnTo = requestedReturn.startsWith('/audit-events')
    ? requestedReturn
    : '/audit-events'

  return (
    <section
      className="mx-auto w-full max-w-300"
      aria-labelledby="audit-event-heading"
    >
      <Link className="text-admin-accent" to={returnTo}>
        ← Back to audit log
      </Link>
      <p className="text-admin-note text-admin-muted-blue m-0 font-mono font-bold tracking-[0.13em]">
        AUDIT EVENT
      </p>
      <h2
        id="audit-event-heading"
        className="mt-2 text-xl font-bold text-white"
      >
        Audit event
      </h2>
      {loading ? (
        <p className="text-admin-muted mt-5" aria-live="polite">
          Loading audit event...
        </p>
      ) : null}
      {message ? <Notice variant="error">{message}</Notice> : null}
      {event ? (
        <div className="border-admin-control-border bg-admin-control-panel-alt text-admin-control-muted-strong mt-5 grid gap-5 border p-5 text-sm">
          <dl className="grid grid-cols-2 gap-5 max-[620px]:grid-cols-1">
            <div>
              <dt className="font-bold text-white">Action</dt>
              <dd>{event.action}</dd>
            </div>
            <div>
              <dt className="font-bold text-white">Actor ID</dt>
              <dd className="break-all">{event.actor_id || 'System'}</dd>
            </div>
            <div>
              <dt className="font-bold text-white">Resource</dt>
              <dd>{event.resource_type || 'None'}</dd>
            </div>
            <div>
              <dt className="font-bold text-white">Resource ID</dt>
              <dd className="break-all">{event.resource_id || 'None'}</dd>
            </div>
            <div>
              <dt className="font-bold text-white">Outcome</dt>
              <dd>{event.outcome}</dd>
            </div>
            <div>
              <dt className="font-bold text-white">Reason</dt>
              <dd>{event.reason || 'None'}</dd>
            </div>
            <div>
              <dt className="font-bold text-white">Occurred</dt>
              <dd>{new Date(event.occurred_at).toLocaleString()}</dd>
            </div>
            <div>
              <dt className="font-bold text-white">Event ID</dt>
              <dd className="break-all">{event.id}</dd>
            </div>
            <div>
              <dt className="font-bold text-white">Request ID</dt>
              <dd className="break-all">{event.request_id || 'None'}</dd>
            </div>
            <div>
              <dt className="font-bold text-white">IP address</dt>
              <dd>{event.ip_address || 'Not recorded'}</dd>
            </div>
          </dl>
          <JsonDetail title="Before state" value={event.before_state} />
          <JsonDetail title="After state" value={event.after_state} />
          <JsonDetail title="Metadata" value={event.metadata} />
        </div>
      ) : null}
    </section>
  )
}

function JsonDetail({ title, value }: { title: string; value: unknown }) {
  if (value === undefined || value === null) return null
  return (
    <details className="border-admin-border-divider border-t pt-4">
      <summary className="cursor-pointer font-bold text-white">{title}</summary>
      <pre className="bg-admin-canvas text-admin-muted mt-3 overflow-x-auto p-4 text-xs">
        {JSON.stringify(value, null, 2)}
      </pre>
    </details>
  )
}
