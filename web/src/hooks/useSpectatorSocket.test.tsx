import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { useSpectatorSocket } from './useSpectatorSocket'

class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 3
  static instances: MockWebSocket[] = []

  readyState = MockWebSocket.CONNECTING
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent<string>) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null

  constructor() {
    MockWebSocket.instances.push(this)
  }

  send = vi.fn()
  addEventListener = vi.fn()
  close = vi.fn(() => { this.readyState = MockWebSocket.CLOSED })
}

beforeEach(() => {
  MockWebSocket.instances = []
  vi.stubGlobal('WebSocket', MockWebSocket)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

test('uses the unavailable-game state when spectator access is rejected by the server', () => {
  const { result, unmount } = renderHook(() => useSpectatorSocket('room-1', 'token'))
  const socket = MockWebSocket.instances[0]

  act(() => {
    socket.onmessage?.({
      data: JSON.stringify({ type: 'error', message: 'spectator access is temporarily unavailable' }),
    } as MessageEvent<string>)
  })

  expect(result.current.notFound).toBe(true)
  unmount()
})
