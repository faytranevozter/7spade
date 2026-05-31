import { useEffect, useRef, useState } from 'react'
import { emotes } from '../game/emotes'

// EmotePicker is a small floating button that opens a tray of emotes. Selecting
// one calls onSelect with the emote id and closes the tray.
export function EmotePicker({ onSelect }: { onSelect: (id: string) => void }) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  // Close on outside click or Escape, matching the Modal affordances.
  useEffect(() => {
    if (!open) return
    const onPointerDown = (event: PointerEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false)
      }
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    window.addEventListener('pointerdown', onPointerDown)
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  const choose = (id: string) => {
    onSelect(id)
    setOpen(false)
  }

  return (
    <div ref={containerRef} className="relative">
      {open ? (
        <div
          role="menu"
          aria-label="Emotes"
          className="absolute bottom-full right-0 mb-2 grid w-44 grid-cols-3 gap-1 rounded-spade-lg border border-spade-cream/12 bg-spade-bg/95 p-2 shadow-spade-card backdrop-blur"
        >
          {emotes.map((emote) => {
            const isWord = /[a-zA-Z]/.test(emote.glyph)
            return (
              <button
                key={emote.id}
                type="button"
                role="menuitem"
                title={emote.label}
                aria-label={emote.label}
                onClick={() => choose(emote.id)}
                className={`grid h-10 place-items-center rounded-spade-md border border-transparent bg-spade-cream/5 transition hover:border-spade-gold/40 hover:bg-spade-cream/10 ${
                  isWord ? 'text-xs font-semibold text-spade-gold-light' : 'text-xl leading-none'
                }`}
              >
                {emote.glyph}
              </button>
            )
          })}
        </div>
      ) : null}
      <button
        type="button"
        aria-label="Open emotes"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
        className="grid size-10 place-items-center rounded-full border border-spade-cream/15 bg-spade-bg/80 text-xl leading-none shadow-spade-card transition hover:border-spade-gold/40 hover:bg-spade-cream/10"
      >
        😊
      </button>
    </div>
  )
}
