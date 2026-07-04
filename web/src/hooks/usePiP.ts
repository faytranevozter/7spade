import { useCallback, useEffect, useRef, useState } from 'react'

interface DocumentPictureInPicture {
  requestWindow(options?: { width?: number; height?: number }): Promise<Window>
  window: Window | null
}

declare global {
  interface Window {
    documentPictureInPicture?: DocumentPictureInPicture
  }
}

export function usePiP() {
  const [enabled, setEnabled] = useState(false)
  const [isOpen, setIsOpen] = useState(false)
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  const pipWindowRef = useRef<Window | null>(null)
  const openingRef = useRef(false)

  const isSupported = typeof window !== 'undefined' && 'documentPictureInPicture' in window

  const openWindow = useCallback(async () => {
    if (!isSupported) return
    if (pipWindowRef.current && !pipWindowRef.current.closed) return
    if (openingRef.current) return
    openingRef.current = true

    try {
      const pipWindow = await window.documentPictureInPicture!.requestWindow({
        width: 320,
        height: 260,
      })

      ;[...document.styleSheets].forEach((styleSheet) => {
        try {
          const cssRules = [...styleSheet.cssRules].map((rule) => rule.cssText).join('')
          const style = document.createElement('style')
          style.textContent = cssRules
          pipWindow.document.head.appendChild(style)
        } catch {
          if (styleSheet.href) {
            const link = document.createElement('link')
            link.rel = 'stylesheet'
            link.href = styleSheet.href
            pipWindow.document.head.appendChild(link)
          }
        }
      })

      pipWindow.document.body.style.margin = '0'
      pipWindow.document.body.style.overflow = 'hidden'

      const root = document.createElement('div')
      root.id = 'pip-root'
      pipWindow.document.body.appendChild(root)

      pipWindowRef.current = pipWindow
      setContainer(root)
      setIsOpen(true)

      pipWindow.addEventListener('pagehide', () => {
        pipWindowRef.current = null
        setContainer(null)
        setIsOpen(false)
        setEnabled(false)
        openingRef.current = false
      }, { once: true })
    } catch {
      setIsOpen(false)
    }
    openingRef.current = false
  }, [isSupported])

  const closeWindow = useCallback(() => {
    if (pipWindowRef.current && !pipWindowRef.current.closed) {
      pipWindowRef.current.close()
    }
    pipWindowRef.current = null
    setContainer(null)
    setIsOpen(false)
    openingRef.current = false
  }, [])

  const enable = useCallback(async () => {
    setEnabled(true)
    await openWindow()
  }, [openWindow])

  const disable = useCallback(() => {
    setEnabled(false)
    closeWindow()
  }, [closeWindow])

  useEffect(() => {
    return () => {
      if (pipWindowRef.current && !pipWindowRef.current.closed) {
        pipWindowRef.current.close()
      }
    }
  }, [])

  return { isSupported, enabled, isOpen, openWindow, closeWindow, enable, disable, container }
}
