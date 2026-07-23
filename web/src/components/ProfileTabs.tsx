import { useId, type ReactNode } from 'react'
import { useSearchParams } from 'react-router'

export type ProfileTab = {
  id: string
  label: string
  panel: ReactNode
}

type ProfileTabsProps = {
  tabs: ProfileTab[]
  // URL search-param key used to persist the selected tab (e.g. ?tab=rating).
  paramKey?: string
}

// ProfileTabs is a lightweight, accessible tab strip. The selected tab is
// persisted in the URL query string so a refresh or shared link reopens the
// same view; an unknown/missing value falls back to the first tab. The strip
// sticks under the viewport top while scrolling long panels.
export function ProfileTabs({ tabs, paramKey = 'tab' }: ProfileTabsProps) {
  const [searchParams, setSearchParams] = useSearchParams()
  const baseId = useId()

  const requested = searchParams.get(paramKey)
  const active = tabs.find((t) => t.id === requested) ?? tabs[0]

  const select = (id: string) => {
    const next = new URLSearchParams(searchParams)
    next.set(paramKey, id)
    setSearchParams(next, { replace: true })
  }

  const onKeyDown = (event: React.KeyboardEvent, index: number) => {
    if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return
    event.preventDefault()
    const delta = event.key === 'ArrowRight' ? 1 : -1
    const nextIndex = (index + delta + tabs.length) % tabs.length
    select(tabs[nextIndex].id)
  }

  return (
    <div className="grid gap-4">
      <div
        role="tablist"
        aria-label="Profile sections"
        className="sticky top-0 z-10 -mx-4 flex flex-wrap gap-2 border-b border-spade-cream/10 bg-[#102316] px-4 py-3 sm:-mx-5 sm:px-5"
      >
        {tabs.map((tab, index) => {
          const selected = tab.id === active.id
          return (
            <button
              key={tab.id}
              role="tab"
              id={`${baseId}-tab-${tab.id}`}
              aria-selected={selected}
              aria-controls={`${baseId}-panel-${tab.id}`}
              tabIndex={selected ? 0 : -1}
              onClick={() => select(tab.id)}
              onKeyDown={(e) => onKeyDown(e, index)}
              className={`inline-flex min-h-9 items-center rounded-spade-pill border px-4 py-1.5 text-sm font-medium transition ${
                selected
                  ? 'border-spade-gold-light bg-spade-gold text-[#1a0e00]'
                  : 'border-spade-cream/12 bg-spade-bg/45 text-spade-gray-2 hover:border-spade-gold/45 hover:text-spade-cream'
              }`}
            >
              {tab.label}
            </button>
          )
        })}
      </div>
      <div
        role="tabpanel"
        id={`${baseId}-panel-${active.id}`}
        aria-labelledby={`${baseId}-tab-${active.id}`}
      >
        {active.panel}
      </div>
    </div>
  )
}
