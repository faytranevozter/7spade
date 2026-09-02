import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { createSkin, skinTypeLabel } from '../api/skins'
import { useAuth } from '../hooks/useAuth'

const skinTypes = [
  'profile_background',
  'avatar_frame',
  'display_picture',
  'player_card_background',
]
const inputClass =
  'rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus w-full min-w-0 border outline-none'

export function SkinCreatePage() {
  const { token } = useAuth()
  const navigate = useNavigate()
  const [draft, setDraft] = useState({
    name: '',
    skin_type: skinTypes[0],
    description: '',
    display_order: 0,
    reason: '',
  })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!token || saving) return
    setSaving(true)
    setError('')
    try {
      const skin = await createSkin(token, draft)
      navigate(`/skins/${skin.id}`)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to create skin')
    } finally {
      setSaving(false)
    }
  }
  return (
    <form onSubmit={submit} className="mx-auto w-full max-w-360">
      <Link
        to="/skins"
        className="text-admin-accent hover:text-admin-accent-bright text-admin-field mb-4 inline-block"
      >
        ← Back to skins
      </Link>
      <header className="border-admin-border flex items-end justify-between gap-8 border-b pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <div>
          <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
            Content catalog
          </p>
          <h1 className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-hero leading-[0.95] font-medium tracking-[-0.06em]">
            Create skin
          </h1>
          <p className="text-admin-muted m-0 max-w-170 leading-[1.65]">
            Create a disabled, hidden catalog draft. Asset upload, eligibility,
            and publication happen in the new record.
          </p>
        </div>
        <span className="text-admin-muted-subtle text-admin-caption self-end">
          Draft workflow
        </span>
      </header>
      {error && (
        <div
          role="alert"
          className="text-admin-danger border-admin-danger-border bg-admin-danger-bg my-4 rounded-lg border px-4 py-3"
        >
          {error}
        </div>
      )}
      <div className="gap-admin-17 mt-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,360px)] items-start max-[1050px]:grid-cols-1">
        <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5 max-[500px]:p-4">
          <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
            Catalog record
          </p>
          <h2 className="text-admin-ink-strong text-admin-heading mt-admin-4 mb-5">
            Draft details
          </h2>
          <div className="grid grid-cols-[minmax(0,1fr)_180px] gap-4 max-[760px]:grid-cols-1">
            <Field label="Name">
              <input
                className={inputClass}
                aria-label="New skin name"
                value={draft.name}
                onChange={(event) =>
                  setDraft({ ...draft, name: event.target.value })
                }
              />
            </Field>
            <Field label="Skin type">
              <select
                className={inputClass}
                aria-label="New skin type"
                value={draft.skin_type}
                onChange={(event) =>
                  setDraft({ ...draft, skin_type: event.target.value })
                }
              >
                {skinTypes.map((type) => (
                  <option key={type} value={type}>
                    {skinTypeLabel(type)}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Description" wide>
              <textarea
                className={`${inputClass} min-h-32 resize-y`}
                aria-label="New skin description"
                value={draft.description}
                onChange={(event) =>
                  setDraft({ ...draft, description: event.target.value })
                }
              />
            </Field>
            <Field label="Display order">
              <input
                className={inputClass}
                aria-label="New skin display order"
                type="number"
                min="0"
                value={draft.display_order}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    display_order: Number(event.target.value),
                  })
                }
              />
            </Field>
          </div>
        </section>
        <aside className="grid gap-5">
          <section className="rounded-admin-preview border-admin-accent-border-faint bg-admin-surface-preview border p-5">
            <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
              Draft status
            </p>
            <h2 className="text-admin-ink-strong text-admin-heading mt-admin-4 mb-3">
              What happens next
            </h2>
            <ol className="text-admin-muted m-0 grid gap-3 pl-5 leading-[1.55]">
              <li>The new skin starts disabled and hidden.</li>
              <li>Upload and preview its asset from the skin record.</li>
              <li>
                Publish an asset revision, then enable catalog availability.
              </li>
            </ol>
          </section>
          <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5">
            <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
              Audit record
            </p>
            <h2 className="text-admin-ink-strong text-admin-heading mt-admin-4 mb-5">
              Create draft
            </h2>
            <Field label="Creation reason">
              <textarea
                className={`${inputClass} min-h-24 resize-y`}
                aria-label="Creation reason"
                value={draft.reason}
                placeholder="Why is this catalog record needed?"
                onChange={(event) =>
                  setDraft({ ...draft, reason: event.target.value })
                }
              />
            </Field>
            <button
              className="bg-admin-accent border-admin-accent-border text-admin-button-ink rounded-admin-input mt-4 w-full cursor-pointer border px-4 py-3 font-semibold disabled:cursor-not-allowed disabled:opacity-50"
              disabled={saving || !draft.name.trim() || !draft.reason.trim()}
              type="submit"
            >
              {saving ? 'Creating draft...' : 'Create draft'}
            </button>
          </section>
        </aside>
      </div>
    </form>
  )
}

function Field({
  label,
  children,
  wide = false,
}: {
  label: string
  children: React.ReactNode
  wide?: boolean
}) {
  return (
    <label
      className={`gap-admin-5 text-admin-field text-admin-muted grid min-w-0 ${wide ? 'col-span-full max-[760px]:col-auto' : ''}`}
    >
      <span className="text-admin-label font-mono tracking-wider uppercase">
        {label}
      </span>
      {children}
    </label>
  )
}
