import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useGameSocket } from './useGameSocket'

class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  static instances: MockWebSocket[] = []

  readyState = MockWebSocket.CONNECTING
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent<string>) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null

  constructor(public url: string) {
    MockWebSocket.instances.push(this)
  }

  send = vi.fn()
  addEventListener = vi.fn((type: string, handler: EventListener) => {
    if (type === 'open') this.onopen = handler as (event: Event) => void
  })

  close() {
    this.readyState = MockWebSocket.CLOSED
  }

  openFromServer() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.(new Event('open'))
  }

  closeFromServer() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({} as CloseEvent)
  }
}

describe('useGameSocket reconnect', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    MockWebSocket.instances = []
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('automatically opens a new socket after a transient close', async () => {
    const { unmount } = renderHook(() => useGameSocket('room-1', 'token'))

    expect(MockWebSocket.instances).toHaveLength(1)

    await act(async () => {
      MockWebSocket.instances[0].openFromServer()
      MockWebSocket.instances[0].closeFromServer()
      await vi.advanceTimersByTimeAsync(1000)
    })

    expect(MockWebSocket.instances).toHaveLength(2)
    expect(MockWebSocket.instances[1].url).toContain('room_id=room-1')

    unmount()
  })

  it('does not reconnect after unmount', async () => {
    const { unmount } = renderHook(() => useGameSocket('room-1', 'token'))

    expect(MockWebSocket.instances).toHaveLength(1)

    await act(async () => {
      MockWebSocket.instances[0].openFromServer()
      MockWebSocket.instances[0].closeFromServer()
      unmount()
      await vi.advanceTimersByTimeAsync(1000)
    })

    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('clears a pending reconnect when the room becomes unavailable', async () => {
    const { rerender } = renderHook(
      ({ roomId, token }) => useGameSocket(roomId, token),
      { initialProps: { roomId: 'room-1' as string | undefined, token: 'token' as string | null } },
    )

    await act(async () => {
      MockWebSocket.instances[0].openFromServer()
      MockWebSocket.instances[0].closeFromServer()
      rerender({ roomId: undefined, token: 'token' })
      await vi.advanceTimersByTimeAsync(1000)
    })

    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('reconnects immediately when a send throws during a network handoff', async () => {
    const { result, unmount } = renderHook(() => useGameSocket('room-1', 'token'))

    await act(async () => {
      MockWebSocket.instances[0].openFromServer()
    })
    MockWebSocket.instances[0].send.mockImplementationOnce(() => {
      throw new Error('socket closed')
    })

    await act(async () => {
      result.current.sendEmote('gg')
    })

    expect(MockWebSocket.instances).toHaveLength(2)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000)
    })

    expect(MockWebSocket.instances).toHaveLength(2)

    unmount()
  })
})
