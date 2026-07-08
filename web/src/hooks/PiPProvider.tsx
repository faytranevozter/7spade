import { createContext, useContext, type ReactNode } from 'react'
import { usePiP } from './usePiP'

type PiPContextValue = ReturnType<typeof usePiP>

const PiPContext = createContext<PiPContextValue | null>(null)

const NOOP_PIP: PiPContextValue = {
  isSupported: false,
  enabled: false,
  isOpen: false,
  openWindow: async () => {},
  closeWindow: () => {},
  enable: async () => {},
  disable: () => {},
  container: null,
}

// eslint-disable-next-line react-refresh/only-export-components
export function usePiPContext(): PiPContextValue {
  return useContext(PiPContext) ?? NOOP_PIP
}

export function PiPProvider({ children }: { children: ReactNode }) {
  const pip = usePiP()
  return <PiPContext.Provider value={pip}>{children}</PiPContext.Provider>
}
