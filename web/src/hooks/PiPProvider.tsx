import { createContext, useContext, type ReactNode } from 'react'
import { usePiP } from './usePiP'

type PiPContextValue = ReturnType<typeof usePiP>

const PiPContext = createContext<PiPContextValue | null>(null)

// eslint-disable-next-line react-refresh/only-export-components
export function usePiPContext(): PiPContextValue {
  const context = useContext(PiPContext)
  if (!context) {
    throw new Error('usePiPContext must be used within a PiPProvider')
  }
  return context
}

export function PiPProvider({ children }: { children: ReactNode }) {
  const pip = usePiP()
  return <PiPContext.Provider value={pip}>{children}</PiPContext.Provider>
}
