import { useEffect, type ReactNode } from 'react'

type ModalTone = 'default' | 'danger'

type ModalProps = {
  title: string
  eyebrow?: string
  description?: string
  tone?: ModalTone
  children: ReactNode
  footer?: ReactNode
  onClose?: () => void
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
  onClose,
}: ModalProps) {
  const titleId = `${title.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-modal-title`

  // Escape closes the modal, matching the backdrop-click affordance.
  useEffect(() => {
    if (!onClose) return undefined
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        aria-labelledby={titleId}
        aria-modal="true"
        className={`w-full max-w-[420px] rounded-spade-lg border ${toneClasses[tone]} bg-spade-gray-4 p-5 shadow-[0_24px_80px_rgba(0,0,0,0.38)]`}
        role="dialog"
        onClick={(event) => event.stopPropagation()}
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
          {onClose ? (
            <button
              aria-label={`Close ${title}`}
              onClick={onClose}
              className="grid size-8 shrink-0 place-items-center rounded-spade-sm border border-spade-cream/15 text-spade-gray-2 transition hover:bg-spade-cream/8 hover:text-spade-cream"
              type="button"
            >
              x
            </button>
          ) : null}
        </div>

        <div className="mt-5">{children}</div>

        {footer ? (
          <div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            {footer}
          </div>
        ) : null}
      </div>
    </div>
  )
}
