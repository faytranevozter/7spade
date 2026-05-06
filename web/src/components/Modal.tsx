import type { ReactNode } from 'react'

type ModalTone = 'default' | 'danger'

type ModalProps = {
  title: string
  eyebrow?: string
  description?: string
  tone?: ModalTone
  children: ReactNode
  footer?: ReactNode
}

const toneClasses: Record<ModalTone, string> = {
  default: 'border-spade-green-light/25',
  danger: 'border-spade-red/35',
}

export function Modal({
  title,
  eyebrow,
  description,
  tone = 'default',
  children,
  footer,
}: ModalProps) {
  const titleId = `${title.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-modal-title`

  return (
    <div className="rounded-spade-lg border border-spade-gray-4/50 bg-spade-bg/80 p-3">
      <div className="grid min-h-[320px] place-items-center rounded-spade-md bg-card-back p-4">
        <div
          aria-labelledby={titleId}
          aria-modal="true"
          className={`w-full max-w-[420px] rounded-spade-lg border ${toneClasses[tone]} bg-spade-gray-4 p-5 shadow-[0_24px_80px_rgba(0,0,0,0.38)]`}
          role="dialog"
        >
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              {eyebrow ? (
                <p className="mb-2 text-xs font-semibold uppercase tracking-[0.08em] text-spade-gold">
                  {eyebrow}
                </p>
              ) : null}
              <h3 id={titleId} className="text-xl font-medium text-spade-cream">
                {title}
              </h3>
              {description ? (
                <p className="mt-2 max-w-[34ch] text-sm leading-5 text-spade-gray-2">
                  {description}
                </p>
              ) : null}
            </div>
            <button
              aria-label={`Close ${title}`}
              className="grid size-8 shrink-0 place-items-center rounded-spade-sm border border-spade-cream/15 text-spade-gray-2 transition hover:bg-spade-cream/8 hover:text-spade-cream"
              type="button"
            >
              x
            </button>
          </div>

          <div className="mt-5">{children}</div>

          {footer ? (
            <div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              {footer}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )
}
