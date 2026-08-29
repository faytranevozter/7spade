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
      className={`border-admin-accent bg-admin-accent/9 my-4 rounded-r-md border-l-[3px] px-[0.9rem] py-3 text-[0.78rem] text-[#e0b45e] ${variant === 'error' ? 'border-[#c0392b] bg-[#c0392b]/10 text-[#ffaaa4]' : variant === 'success' ? 'border-[#2d7a46] bg-[#2d7a46]/12 text-[#92d3a5]' : ''} ${className}`.trim()}
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
      <p className="text-admin-muted-subtle mt-[-0.45rem] mr-0 mb-0 ml-0 text-[0.7rem]">
        {children}
      </p>
    </div>
  )
}

export function LoadingState({ children }: { children: ReactNode }) {
  return (
    <p className="text-admin-muted-subtle text-[0.75rem]" aria-live="polite">
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
    <div className="border-admin-accent/38 bg-admin-accent/8 mt-4 flex items-center justify-between gap-4 rounded-[9px] border px-4 py-[0.85rem]">
      <div>
        <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
          {eyebrow}
        </p>
        <strong className="text-admin-accent-bright my-[0.35rem] block font-mono text-[0.72rem] break-all">
          {credential}
        </strong>
        <small className="text-admin-muted block text-[0.62rem]">
          {description}
        </small>
      </div>
      <button
        type="button"
        onClick={onDismiss}
        className="border-admin-ink/16 text-admin-muted cursor-pointer rounded-[5px] border bg-transparent px-[0.65rem] py-2 text-[0.62rem]"
      >
        Dismiss
      </button>
    </div>
  )
}
