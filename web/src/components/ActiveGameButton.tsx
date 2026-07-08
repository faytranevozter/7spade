import { useLocation, useNavigate } from 'react-router'
import { useActiveRoom } from '../hooks/useActiveRoom'

// ActiveGameButton floats over every authed page to let a player jump back into
// the game they're currently in. It hides itself while the player is already on
// that room's page (waiting room or live game), so it only nudges
// when they've wandered off (lobby, history, profile, etc.).
export function ActiveGameButton() {
  const { activeRoom } = useActiveRoom()
  const { pathname } = useLocation()
  const navigate = useNavigate()

  if (!activeRoom) return null

  const destination = activeRoom.status === 'in_progress' ? `/game/${activeRoom.id}` : `/room/${activeRoom.id}`

  // Already viewing this room (any of its pages)? Don't show the nudge.
  const onThisRoom =
    pathname === `/room/${activeRoom.id}` ||
    pathname === `/game/${activeRoom.id}`
  if (onThisRoom) return null

  const label = activeRoom.status === 'in_progress' ? 'Game in progress' : 'Waiting for players'

  return (
    <button
      type="button"
      onClick={() => navigate(destination)}
      aria-label={`Return to your game (${label})`}
      className="fixed bottom-5 right-5 z-50 inline-flex items-center gap-3 rounded-spade-pill border border-spade-gold-light/60 bg-spade-gold px-5 py-3 text-sm font-medium text-[#1a0e00] shadow-[0_0_28px_rgb(201_146_43_/_40%)] transition hover:bg-[#d9a030] active:scale-95"
    >
      <span className="relative flex size-2.5">
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-[#1a0e00]/60" />
        <span className="relative inline-flex size-2.5 rounded-full bg-[#1a0e00]" />
      </span>
      <span className="flex flex-col items-start leading-tight">
        <span className="font-semibold">Resume game</span>
        <span className="font-mono text-[11px] uppercase tracking-[0.1em] opacity-70">{label}</span>
      </span>
    </button>
  )
}
