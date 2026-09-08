import type { ReactNode } from 'react'

export const adminPageClassName = 'mx-auto w-full max-w-360'

export const adminPanelClassName =
  'rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5 max-[500px]:p-4'

export function AdminPage({
  children,
  className = '',
  labelledBy,
}: {
  children: ReactNode
  className?: string
  labelledBy?: string
}) {
  return (
    <section
      className={`${adminPageClassName} ${className}`.trim()}
      aria-labelledby={labelledBy}
    >
      {children}
    </section>
  )
}

export function AdminPageHeader({
  eyebrow,
  title,
  description,
  actions,
  backLink,
  variant = 'page',
  titleId,
}: {
  eyebrow: string
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
  backLink?: ReactNode
  variant?: 'page' | 'detail'
  titleId?: string
}) {
  const titleClass =
    variant === 'detail' ? 'text-admin-detail-hero' : 'text-admin-hero'

  return (
    <>
      {backLink ? <div className="mb-4">{backLink}</div> : null}
      <header className="border-admin-border flex items-end justify-between gap-8 border-b pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <div className="min-w-0">
          <p className="text-admin-accent text-admin-label m-0 font-mono tracking-wider uppercase">
            {eyebrow}
          </p>
          <h1
            id={titleId}
            className={`text-admin-ink-strong mt-admin-7 mb-admin-9 ${titleClass} leading-[0.95] font-medium tracking-[-0.06em]`}
          >
            {title}
          </h1>
          {description ? (
            <div className="text-admin-muted max-w-170 leading-[1.65]">
              {description}
            </div>
          ) : null}
        </div>
        {actions ? <div className="shrink-0 max-[760px]:w-full">{actions}</div> : null}
      </header>
    </>
  )
}

export function AdminPanel({
  children,
  className = '',
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div className={`${adminPanelClassName} ${className}`.trim()}>
      {children}
    </div>
  )
}
