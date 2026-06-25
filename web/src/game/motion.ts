// Motion preference manager for card-play animations.
//
// Mirrors the AudioManager pattern (game/sound.ts): a module singleton that
// persists a preference to localStorage and exposes a subscribe() pub-sub so
// React can mirror it into state. The preference is one of four speeds; "off"
// disables all gameplay animations. The default honours the OS-level
// `prefers-reduced-motion` setting — when the user asked the system to reduce
// motion, we default to "off" — but an explicit choice always wins and is
// remembered.
//
// Animations themselves are pure CSS (keyframes in index.css). This manager
// only owns the *duration scale*, published as the `--anim-scale` CSS variable
// on <html>, which every keyframe/transition multiplies into its duration.
// "off" collapses the scale to 0 so animations resolve instantly (no motion).

export type MotionSpeed = 'off' | 'slow' | 'normal' | 'fast'

export const MOTION_SPEEDS: MotionSpeed[] = ['off', 'slow', 'normal', 'fast']

const MOTION_KEY = 'seven_spade_motion'

// Duration multiplier applied to every animation for each speed. "off" is 0 so
// the CSS resolves to a 0s duration (instant, effectively disabled).
const SCALE: Record<MotionSpeed, number> = {
  off: 0,
  slow: 1.6,
  normal: 1,
  fast: 0.6,
}

function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  try {
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches
  } catch {
    return false
  }
}

function isMotionSpeed(value: unknown): value is MotionSpeed {
  return value === 'off' || value === 'slow' || value === 'normal' || value === 'fast'
}

// readSpeed resolves the initial preference: an explicit stored choice wins;
// otherwise the OS reduced-motion setting decides (off when set, normal else).
function readSpeed(): MotionSpeed {
  try {
    const stored = window.localStorage.getItem(MOTION_KEY)
    if (isMotionSpeed(stored)) return stored
  } catch {
    // Ignore storage failures (private mode, SSR); fall through to OS default.
  }
  return prefersReducedMotion() ? 'off' : 'normal'
}

class MotionManager {
  private speed: MotionSpeed = readSpeed()
  private hasExplicitChoice = false
  private readonly listeners = new Set<(speed: MotionSpeed) => void>()

  constructor() {
    // Record whether the initial value came from an explicit stored choice, so
    // a later OS reduced-motion change only overrides an *un-chosen* default.
    try {
      this.hasExplicitChoice = isMotionSpeed(window.localStorage.getItem(MOTION_KEY))
    } catch {
      this.hasExplicitChoice = false
    }
    this.applyToDocument()
    this.watchReducedMotion()
  }

  getSpeed(): MotionSpeed {
    return this.speed
  }

  // enabled is true unless the user (or the OS default) turned animations off.
  isEnabled(): boolean {
    return this.speed !== 'off'
  }

  // durationScale is the multiplier the CSS uses; 0 when off.
  durationScale(): number {
    return SCALE[this.speed]
  }

  setSpeed(speed: MotionSpeed): void {
    this.speed = speed
    this.hasExplicitChoice = true
    try {
      window.localStorage.setItem(MOTION_KEY, speed)
    } catch {
      // In-memory state still works without persistence.
    }
    this.applyToDocument()
    for (const listener of this.listeners) listener(speed)
  }

  // cycle advances to the next speed (Off -> Slow -> Normal -> Fast -> Off),
  // used by the single-button header control.
  cycle(): void {
    const index = MOTION_SPEEDS.indexOf(this.speed)
    const next = MOTION_SPEEDS[(index + 1) % MOTION_SPEEDS.length]
    this.setSpeed(next)
  }

  subscribe(listener: (speed: MotionSpeed) => void): () => void {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  // applyToDocument publishes the scale as a CSS variable + a data attribute so
  // stylesheets can both compute durations and target [data-motion="off"].
  private applyToDocument(): void {
    if (typeof document === 'undefined') return
    const root = document.documentElement
    root.style.setProperty('--anim-scale', String(SCALE[this.speed]))
    root.dataset.motion = this.speed
  }

  // watchReducedMotion keeps an un-chosen default in sync with the OS setting:
  // if the player never picked a speed and toggles reduced-motion at the OS
  // level, flip between off and normal to match.
  private watchReducedMotion(): void {
    if (typeof window === 'undefined' || !window.matchMedia) return
    let query: MediaQueryList
    try {
      query = window.matchMedia('(prefers-reduced-motion: reduce)')
    } catch {
      return
    }
    const handler = (event: MediaQueryListEvent) => {
      if (this.hasExplicitChoice) return
      this.speed = event.matches ? 'off' : 'normal'
      this.applyToDocument()
      for (const listener of this.listeners) listener(this.speed)
    }
    if (typeof query.addEventListener === 'function') {
      query.addEventListener('change', handler)
    }
  }
}

// Module singleton so every consumer shares the same preference + CSS var.
export const motionManager = new MotionManager()
