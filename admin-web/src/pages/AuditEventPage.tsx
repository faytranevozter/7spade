import { useEffect, useState } from 'react'
import { useParams } from 'react-router'
import { getAuditEvent, type AuditEvent } from '../api/audit'
import { Notice } from '../components/Feedback'
import { useAuth } from '../hooks/useAuth'

export function AuditEventPage() {
  const { id = '' } = useParams()
  const { token } = useAuth()
  const [event, setEvent] = useState<AuditEvent | null>(null)
  const [message, setMessage] = useState('Loading audit event...')

  useEffect(() => {
    if (!token || !id) return
    getAuditEvent(token, id).then((result) => {
      setEvent(result ?? null)
      setMessage(result ? '' : 'Audit event not found.')
    }).catch((error: unknown) => setMessage(error instanceof Error ? error.message : 'Failed to load audit event'))
  }, [id, token])

  return <section aria-labelledby="audit-event-heading">
    <p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">AUDIT EVENT</p>
    <h2 id="audit-event-heading" className="mt-2 text-xl font-bold text-white">Moderation audit</h2>
    {message ? <Notice variant="error">{message}</Notice> : null}
    {event ? <dl className="mt-5 grid gap-3 border border-[#28323d] bg-[#10161d] p-5 text-sm text-[#aeb8c4]">
      <div><dt className="font-bold text-white">Action</dt><dd>{event.action}</dd></div>
      <div><dt className="font-bold text-white">Outcome</dt><dd>{event.outcome}</dd></div>
      <div><dt className="font-bold text-white">Reason</dt><dd>{event.reason || 'None'}</dd></div>
      <div><dt className="font-bold text-white">Occurred</dt><dd>{event.occurred_at}</dd></div>
      <div><dt className="font-bold text-white">Event ID</dt><dd>{event.id}</dd></div>
    </dl> : null}
  </section>
}
