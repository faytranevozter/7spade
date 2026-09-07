import { useEffect, useId, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { getEvents, type AdminEvent } from '../api/events'
import {
  createSkin,
  getAchievements,
  publishSkin,
  saveSkin,
  skinTypeLabel,
  uploadSkinAsset,
  type Achievement,
  type SkinUnlockRule,
} from '../api/skins'
import { useAuth } from '../hooks/useAuth'
import { Notice } from '../components/Feedback'
import { UnlockRules } from './SkinDetailPage'

const skinTypes = [
  'profile_background',
  'avatar_frame',
  'display_picture',
  'player_card_background',
]
const inputClass =
  'rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus w-full min-w-0 border outline-none'
const initialRule: SkinUnlockRule = {
  name: 'New unlock rule',
  rule_type: 'achievement',
  achievement_id: '',
  retroactive: false,
  enabled: true,
}

export function SkinCreatePage() {
  const { token } = useAuth()
  const navigate = useNavigate()
  const fileInputId = useId()
  const imagePreviewURL = useRef('')
  const [draft, setDraft] = useState({
    name: '',
    skin_type: skinTypes[0],
    description: '',
    display_order: 0,
    reason: '',
  })
  const [rules, setRules] = useState<SkinUnlockRule[]>([initialRule])
  const [image, setImage] = useState<File | null>(null)
  const [imagePreview, setImagePreview] = useState('')
  const [achievements, setAchievements] = useState<Achievement[]>([])
  const [events, setEvents] = useState<AdminEvent[]>([])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!token) return
    void getAchievements(token)
      .then(({ achievements }) => setAchievements(achievements))
      .catch(() => setAchievements([]))
    void getEvents(token)
      .then(({ events }) => setEvents(events))
      .catch(() => setEvents([]))
  }, [token])

  useEffect(
    () => () => {
      if (imagePreviewURL.current) URL.revokeObjectURL(imagePreviewURL.current)
    },
    [],
  )

  const selectImage = (nextImage: File | null) => {
    if (imagePreviewURL.current) URL.revokeObjectURL(imagePreviewURL.current)
    imagePreviewURL.current = nextImage ? URL.createObjectURL(nextImage) : ''
    setImage(nextImage)
    setImagePreview(imagePreviewURL.current)
  }

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!token || saving || !image) return
    setSaving(true)
    setError('')
    try {
      const skin = await createSkin(token, { ...draft, unlock_rules: rules })
      const asset = await uploadSkinAsset(token, skin.id, image)
      const revision = await publishSkin(
        token,
        skin.id,
        asset.asset_key,
        asset.content_type,
        draft.reason,
      )
      await saveSkin(
        token,
        {
          ...skin,
          asset_key: revision.asset_key,
          revisions: [revision],
        },
        draft.reason,
        rules,
      )
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
      <header className="border-admin-border border-b pb-8">
        <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
          Content catalog
        </p>
        <h1 className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-hero leading-[0.95] font-medium tracking-[-0.06em]">
          Create skin
        </h1>
        <p className="text-admin-muted m-0 max-w-170 leading-[1.65]">
          An image and at least one unlock rule are required before a skin can
          be created.
        </p>
      </header>
      {error && (
        <Notice variant="error">{error}</Notice>
      )}
      <div className="mt-6 grid grid-cols-[minmax(0,1fr)] gap-5">
        <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5 max-[500px]:p-4">
          <h2 className="text-admin-ink-strong text-admin-heading mt-0 mb-5">
            Skin details
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
        <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card grid gap-4 border p-5 max-[500px]:p-4">
          <div>
            <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
              Asset pipeline
            </p>
            <h2 className="text-admin-ink-strong text-admin-heading mt-admin-4 mb-0">
              Image
            </h2>
          </div>
          <div className="rounded-admin-preview border-admin-border-section bg-admin-surface-preview grid items-center gap-4 border p-4 max-[760px]:grid-cols-1">
            <div className="flex justify-center">
              {imagePreview ? (
                <img
                  className={`border-admin-accent-border-faint grid max-w-full place-items-center rounded-lg border bg-[radial-gradient(circle_at_50%_30%,#28563a,#0d1a12)] object-cover ${draft.skin_type === 'profile_background' ? 'aspect-admin-profile' : draft.skin_type === 'player_card_background' ? 'aspect-admin-player-card max-h-62.5 object-contain' : 'p-admin-16 aspect-square max-h-62.5 object-contain'}`}
                  src={imagePreview}
                  alt="Selected skin preview"
                />
              ) : (
                <div className="border-admin-accent-border-faint text-admin-suit grid aspect-square size-40 place-items-center rounded-lg border bg-[radial-gradient(circle_at_50%_30%,#28563a,#0d1a12)]">
                  ♠
                </div>
              )}
            </div>
            <div className="gap-admin-5 grid">
              <strong className="text-admin-ink text-admin-preview">
                {image ? 'Selected image' : 'Image required'}
              </strong>
              <span className="text-admin-muted text-admin-field leading-normal">
                {image
                  ? 'Review the framing before creating this skin.'
                  : 'Choose an image to prepare this skin for publication.'}
              </span>
            </div>
          </div>
          <div className="rounded-admin-preview border-admin-accent-border-upload gap-admin-4 grid cursor-pointer place-items-center border border-dashed bg-[#c9922b0d] p-6 text-center">
            <span className="text-admin-ink font-semibold">
              Choose skin image
            </span>
            <small className="text-admin-muted-subtle text-admin-meta">
              PNG, JPEG, WebP, or SVG. Required ratio:{' '}
              {draft.skin_type === 'profile_background'
                ? '10:7'
                : draft.skin_type === 'player_card_background'
                  ? '6:7'
                  : '1:1'}
              .
            </small>
            <input
              className="absolute -m-px size-px overflow-hidden border-0 p-0 whitespace-nowrap [clip:rect(0_0_0_0)]"
              id={fileInputId}
              aria-label="Skin image"
              type="file"
              accept="image/png,image/jpeg,image/webp,image/svg+xml"
              onChange={(event) => selectImage(event.target.files?.[0] ?? null)}
            />
            <label
              className="text-admin-accent-bright focus-within:outline-admin-accent rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 hover:bg-admin-accent-hover inline-flex w-max cursor-pointer border focus-within:outline-2"
              htmlFor={fileInputId}
            >
              {image ? 'Replace image' : 'Select image'}
            </label>
            {image && (
              <span className="text-admin-note text-admin-ink-soft font-mono wrap-anywhere">
                {image.name}
              </span>
            )}
          </div>
        </section>
        <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5 max-[500px]:p-4">
          <h2 className="text-admin-ink-strong text-admin-heading mt-0 mb-5">
            Unlock rules
          </h2>
          <UnlockRules
            rules={rules}
            achievements={achievements}
            events={events}
            editable
            onChange={setRules}
          />
        </section>
        <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5 max-[500px]:p-4">
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
            disabled={
              saving ||
              !draft.name.trim() ||
              !draft.reason.trim() ||
              !image ||
              rules.length === 0
            }
            type="submit"
          >
            {saving ? 'Creating skin...' : 'Create skin'}
          </button>
        </section>
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
