import type { Toast, ToastTone } from '../types'

const toneClasses: Record<ToastTone, string> = {
  success: 'border-spade-green-light/30 bg-spade-green-light/10',
  warn: 'border-spade-gold/30 bg-spade-gold/10',
  info: 'border-[#1e4080]/35 bg-[#1e4080]/15',
  error: 'border-spade-red/30 bg-spade-red/10',
}

const icons: Record<ToastTone, string> = {
  success: '✓',
  warn: '◷',
  info: '→',
  error: '×',
}

export function ToastStack({ toasts }: { toasts: Toast[] }) {
  return (
    <div className="grid gap-2">
      {toasts.map((toast) => (
        <article key={toast.title} className={`flex items-start gap-3 rounded-spade-md border p-3 ${toneClasses[toast.tone]}`}>
          <span className="mt-0.5 grid size-5 shrink-0 place-items-center font-mono text-sm text-spade-gold-light">{icons[toast.tone]}</span>
          <div>
            <h3 className="text-sm font-medium">{toast.title}</h3>
            <p className="mt-0.5 text-xs text-spade-gray-2">{toast.body}</p>
          </div>
        </article>
      ))}
    </div>
  )
}
