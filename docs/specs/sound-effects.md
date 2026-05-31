# Spec: Sound Effects & Mute

Status: Implemented
Owner: —
Related: [Architecture](../architecture.md) · [WebSocket Protocol](../websocket.md) · [Roadmap](../roadmap.md)

## 1. Overview

Add lightweight audio cues to the live game so players get non-visual feedback
for the events that matter most — a card landing, their turn starting, the turn
timer running low, and the game ending. A persisted mute toggle lets players
turn it off. The feature is **entirely frontend**; no server or protocol
changes.

### Goals

- Reinforce key game events with short, unobtrusive sounds.
- Give an audible "your turn" and "time running out" cue, complementing the
  existing turn timer.
- Respect browser autoplay policy and let players mute/unmute (persisted).

### Non-goals

- Background music or ambient loops.
- Server-driven audio, voice chat, or per-emote custom sounds (emote sound is an
  optional stretch, see §7).
- Volume mixing UI beyond a single mute toggle (a volume slider is future work).

## 2. Cues

| Cue | Trigger | Notes |
|---|---|---|
| `card_play` | A card is added to the board (`state_update` board grew) | Plays for self and opponents' plays |
| `your_turn` | `current_turn` becomes the local player | Distinct, attention-grabbing |
| `timer_warning` | Turn timer crosses ~5s remaining on the local player's turn | Fires once per turn |
| `facedown` | A face-down placement (hand shrank, board unchanged) | Softer "thud" |
| `win` / `lose` | `game_over` received | Chosen by the local player's `is_winner` |
| `emote` (optional) | An `emote` message arrives | Off by default; see §7 |

Cues are short (<1s), normalized in volume, and preloaded.

## 3. Frontend Design (`web`)

### Asset loading

- **Implementation note:** the spec's "asset sourcing" open question (below) was
  resolved by **synthesizing** each cue with the Web Audio API rather than
  shipping audio files — no binary assets, no licensing concern. Each cue is a
  short sequence of oscillator tones defined in `web/src/game/sound.ts`.
- The public surface still matches the spec (`AudioManager.play(cue)`, mute,
  autoplay unlock), so swapping in real samples later is a localized change
  inside the manager.

### Autoplay policy

Browsers block audio until a user gesture. The manager stays "locked" until the
first interaction (e.g. the player clicking a card / a ready button), then
unlocks. Cues requested while locked are dropped, not queued.

### Mute state

- `useSound` hook wraps the manager and a `muted` flag stored in `localStorage`
  (key `seven_spade_muted`), defaulting to **unmuted**.
- A small speaker/mute button in the header (`App.tsx`) toggles it, mirroring the
  existing nav controls. The toggle is visible app-wide but only meaningful
  in-game.

### Triggering

Cues fire from the existing `useGameSocket` message flow rather than scattering
audio calls across components:

- `state_update`: diff vs. previous state to detect a board growth (`card_play`),
  a hand-only shrink (`facedown`), and a `current_turn` flip to the local player
  (`your_turn`).
- `game_over`: `win` or `lose` from `is_winner`.
- The turn clock (`useTurnClock`) fires `timer_warning` once when it crosses the
  threshold on the local player's turn.

`useSound` is consumed by `GamePage`; the manager is a module singleton so the
same pool is reused across renders.

## 4. Edge Cases

- **Rapid bot turns**: debounce/coalesce `card_play` so a burst of auto-plays
  doesn't machine-gun the speaker.
- **Backgrounded tab**: browsers throttle timers/audio; cues simply play late or
  are skipped — acceptable.
- **Missing/failed assets**: manager no-ops; the game is fully playable silent.
- **Mute persistence**: survives reload via `localStorage`; defaults unmuted but
  the first cue only plays after the autoplay unlock gesture.
- **Reduced-motion / accessibility**: audio is independent of motion, but the
  mute control gives an explicit opt-out.

## 5. Testing

- `AudioManager`: `play` no-ops when muted or locked; unlocks on gesture; missing
  asset doesn't throw (mock `HTMLAudioElement`).
- `useSound`: mute toggle reads/writes `localStorage`; default unmuted.
- Cue triggering: a `state_update` that grows the board calls `card_play`; a
  turn flip to the local player calls `your_turn`; `game_over` calls `win`/`lose`
  per `is_winner`. (Mock the manager and assert calls.)
- Run `cd web && npm test && npm run lint && npm run build`.

## 6. Rollout

1. Add audio assets + `AudioManager` + `useSound` (muted toggle in header).
2. Wire cue triggers into `useGameSocket` / turn clock.
3. No backend or breaking changes; ships independently.

## 7. Open Questions / Future Work

- **Asset sourcing / licensing** — RESOLVED for v1 by synthesizing cues with the
  Web Audio API (no files). Swapping in recorded/commissioned samples later is a
  localized change inside `AudioManager` (the `play(cue)` surface is unchanged).
- **Volume slider** — a per-category or master volume control beyond mute.
- **Emote sounds** — optional playful sound per emote; off by default to avoid
  spam, gated behind its own toggle.
- **Haptics** — vibration on mobile for `your_turn` / `timer_warning`.
