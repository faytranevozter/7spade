import type { InputHTMLAttributes, ReactNode } from 'react'

type ToggleFieldProps = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'type' | 'className'
> & {
  label: ReactNode
  description?: ReactNode
  className?: string
  showState?: boolean
}

export function ToggleField({
  label,
  description,
  className = '',
  showState = false,
  checked,
  disabled,
  ...props
}: ToggleFieldProps) {
  return (
    <label
      className={`border-admin-border-faint bg-admin-canvas/35 has-checked:border-admin-success-border has-checked:bg-admin-success-bg flex cursor-pointer items-center justify-between gap-5 rounded-lg border p-4 transition-colors has-disabled:cursor-not-allowed has-disabled:opacity-60 ${className}`}
    >
      <span className="min-w-0">
        <strong className="text-admin-ink-strong text-admin-body block font-semibold">
          {label}
        </strong>
        {description ? (
          <small className="text-admin-muted text-admin-caption mt-1 block leading-relaxed">
            {description}
          </small>
        ) : null}
      </span>
      <span className="flex shrink-0 items-center gap-3">
        {showState ? (
          <span className="text-admin-ink-soft text-admin-caption font-medium">
            {checked ? 'On' : 'Off'}
          </span>
        ) : null}
        <span className="relative inline-flex">
          <input
            {...props}
            type="checkbox"
            checked={checked}
            disabled={disabled}
            className="peer sr-only"
          />
          <span className="bg-admin-border-input peer-focus-visible:ring-admin-accent/35 peer-checked:bg-admin-success relative h-7 w-12 rounded-full transition-colors peer-focus-visible:ring-4 peer-disabled:opacity-70 after:absolute after:top-1 after:left-1 after:size-5 after:rounded-full after:bg-white after:shadow-sm after:transition-transform peer-checked:after:translate-x-5" />
        </span>
      </span>
    </label>
  )
}
