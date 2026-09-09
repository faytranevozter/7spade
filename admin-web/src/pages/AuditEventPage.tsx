import { useEffect, useState } from 'react'
import { Link, useSearchParams, useParams } from 'react-router'
import { getAuditEvent, type AuditEvent } from '../api/audit'
import { AdminPage, AdminPageHeader, AdminPanel } from '../components/AdminPage'
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
    <AdminPage labelledBy="audit-event-heading">
      <div className="mx-auto w-full max-w-300">
        <AdminPageHeader
          eyebrow="Audit event"
          title="Audit event"
          titleId="audit-event-heading"
          variant="detail"
          backLink={
            <Link className="text-admin-accent" to={returnTo}>
              ← Back to audit log
            </Link>
          }
        />
        {loading ? (
          <p className="text-admin-muted mt-5" aria-live="polite">
            Loading audit event...
          </p>
        ) : null}
        {message ? <Notice variant="error">{message}</Notice> : null}
        {event ? (
          <AdminPanel className="text-admin-control-muted-strong mt-5 grid gap-5 text-sm">
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
          </AdminPanel>
        ) : null}
      </div>
    </AdminPage>
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
