import { useEffect, useState } from 'react'

// useDebounce returns a copy of value that only updates after delayMs has passed
// without value changing. Used to throttle typeahead searches so we fetch on a
// settled query rather than every keystroke.
export function useDebounce<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(timer)
  }, [value, delayMs])

  return debounced
}
