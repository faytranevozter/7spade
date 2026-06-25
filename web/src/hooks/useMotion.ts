import { useCallback, useEffect, useState } from 'react'
import { motionManager, type MotionSpeed } from '../game/motion'

export type UseMotionReturn = {
  speed: MotionSpeed
  enabled: boolean
  scale: number
  setSpeed: (speed: MotionSpeed) => void
  cycle: () => void
}

// useMotion wraps the MotionManager singleton, tracking the speed as React
// state so toggle UI re-renders. The manager owns persistence (localStorage),
// the prefers-reduced-motion default, and publishing the --anim-scale CSS var.
export function useMotion(): UseMotionReturn {
  const [speed, setSpeedState] = useState<MotionSpeed>(() => motionManager.getSpeed())

  useEffect(() => motionManager.subscribe(setSpeedState), [])

  const setSpeed = useCallback((next: MotionSpeed) => {
    motionManager.setSpeed(next)
  }, [])

  const cycle = useCallback(() => {
    motionManager.cycle()
  }, [])

  return {
    speed,
    enabled: speed !== 'off',
    scale: motionManager.durationScale(),
    setSpeed,
    cycle,
  }
}
