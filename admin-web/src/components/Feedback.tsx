import type { ReactNode } from 'react'

export function Notice({
  children,
  variant = 'info',
  role,
  className = '',
}: {
  children: ReactNode
  variant?: 'info' | 'success' | 'error' | 'warning'
  role?: 'alert' | 'status'
  className?: string
}) {
  const resolvedRole = role ?? (variant === 'error' ? 'alert' : 'status')
  return (
    <p
      role={resolvedRole}
      className={`border-admin-accent bg-admin-accent/9 text-admin-action text-admin-warning px-admin-15 my-4 rounded-r-md border-l-[3px] py-3 ${variant === 'error' ? 'border-admin-danger-border bg-admin-danger-bg text-admin-danger' : variant === 'success' ? 'border-admin-success-border bg-admin-success-bg text-admin-success' : ''} ${className}`.trim()}
    >
      {children}
    </p>
  )
}

export function ReadOnlyNotice({
  title = 'Read-only access',
  children,
}: {
  title?: string
  children: ReactNode
}) {
  return (
    <div>
      <strong className="text-admin-ink-soft text-admin-alert">{title}</strong>
      <p className="text-admin-muted-subtle -mt-admin-6 text-admin-control mr-0 mb-0 ml-0">
        {children}
      </p>
    </div>
  )
}

export function LoadingState({ children }: { children: ReactNode }) {
  return (
    <p
      className="text-admin-muted-subtle text-admin-caption"
      aria-live="polite"
    >
      {children}
    </p>
  )
}

export function CredentialNotice({
  eyebrow,
  credential,
  description,
  onDismiss,
}: {
  eyebrow: string
  credential: string
  description: string
  onDismiss: () => void
}) {
  return (
    <div className="border-admin-accent/38 bg-admin-accent/8 rounded-admin-rule py-admin-14 mt-4 flex items-center justify-between gap-4 border px-4">
      <div>
        <p className="text-admin-accent text-admin-label m-0 font-mono font-medium tracking-[0.13em] uppercase">
          {eyebrow}
        </p>
        <strong className="text-admin-accent-bright my-admin-4 text-admin-button block font-mono break-all">
          {credential}
        </strong>
        <small className="text-admin-muted text-admin-caption block">
          {description}
        </small>
      </div>
      <button
        type="button"
        onClick={onDismiss}
        className="border-admin-ink/16 text-admin-muted px-admin-9 text-admin-caption rounded-admin-control cursor-pointer border bg-transparent py-2"
      >
        Dismiss
      </button>
    </div>
  )
}
