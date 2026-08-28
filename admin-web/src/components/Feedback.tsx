import type { ReactNode } from 'react'

export function Notice({ children, variant = 'info', role, className = '' }: { children: ReactNode; variant?: 'info' | 'success' | 'error' | 'warning'; role?: 'alert' | 'status'; className?: string }) {
  const resolvedRole = role ?? (variant === 'error' ? 'alert' : 'status')
  return <p role={resolvedRole} className={`notice notice-${variant} ${className}`.trim()}>{children}</p>
}

export function ReadOnlyNotice({ title = 'Read-only access', children }: { title?: string; children: ReactNode }) {
  return <div className="read-only-note"><strong>{title}</strong><p>{children}</p></div>
}

export function LoadingState({ children }: { children: ReactNode }) {
  return <p className="directory-loading" aria-live="polite">{children}</p>
}

export function CredentialNotice({ eyebrow, credential, description, onDismiss }: { eyebrow: string; credential: string; description: string; onDismiss: () => void }) {
  return <div className="invite-token-notice">
    <div><p className="eyebrow">{eyebrow}</p><strong>{credential}</strong><small>{description}</small></div>
    <button type="button" onClick={onDismiss}>Dismiss</button>
  </div>
}
