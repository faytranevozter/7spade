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
      className={`my-4 rounded-r-md border-l-[3px] border-admin-accent bg-admin-accent/9 px-[0.9rem] py-3 text-[0.78rem] text-[#e0b45e] ${variant === 'error' ? 'border-[#c0392b] bg-[#c0392b]/10 text-[#ffaaa4]' : variant === 'success' ? 'border-[#2d7a46] bg-[#2d7a46]/12 text-[#92d3a5]' : ''} ${className}`.trim()}
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
      <strong className="text-[0.78rem] text-[#d9d4c8]">{title}</strong>
      <p className="mt-[-0.45rem] mr-0 mb-0 ml-0 text-[0.7rem] text-admin-muted-subtle">
        {children}
      </p>
    </div>
  )
}

export function LoadingState({ children }: { children: ReactNode }) {
  return (
    <p className="text-[0.75rem] text-admin-muted-subtle" aria-live="polite">
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
    <div className="mt-4 flex items-center justify-between gap-4 rounded-[9px] border border-admin-accent/38 bg-admin-accent/8 px-4 py-[0.85rem]">
      <div>
        <p className="m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] text-admin-accent uppercase">
          {eyebrow}
        </p>
        <strong className="my-[0.35rem] block font-mono text-[0.72rem] break-all text-admin-accent-bright">
          {credential}
        </strong>
        <small className="block text-[0.62rem] text-admin-muted">
          {description}
        </small>
      </div>
      <button
        type="button"
        onClick={onDismiss}
        className="cursor-pointer rounded-[5px] border border-admin-ink/16 bg-transparent px-[0.65rem] py-2 text-[0.62rem] text-admin-muted"
      >
        Dismiss
      </button>
    </div>
  )
}
