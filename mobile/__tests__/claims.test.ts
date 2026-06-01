import { decodeJwtClaims } from '../src/auth/claims'

// Builds an unsigned-but-structurally-valid JWT (header.payload.signature) with
// the given payload, base64url-encoded, to exercise the native decoder (which
// doesn't rely on atob).
function makeToken(payload: Record<string, unknown>): string {
  const b64url = (obj: Record<string, unknown>) =>
    Buffer.from(JSON.stringify(obj))
      .toString('base64')
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '')
  return `${b64url({ alg: 'HS256', typ: 'JWT' })}.${b64url(payload)}.signature`
}

describe('decodeJwtClaims', () => {
  it('returns empties for null / malformed tokens', () => {
    expect(decodeJwtClaims(null)).toEqual({ userId: null, displayName: null, avatarUrl: null, isGuest: false })
    expect(decodeJwtClaims('not-a-jwt')).toEqual({ userId: null, displayName: null, avatarUrl: null, isGuest: false })
  })

  it('decodes identity fields from the payload', () => {
    const token = makeToken({
      sub: 'user-123',
      display_name: 'Rini',
      avatar_url: 'https://example.com/a.png',
      is_guest: false,
    })
    expect(decodeJwtClaims(token)).toEqual({
      userId: 'user-123',
      displayName: 'Rini',
      avatarUrl: 'https://example.com/a.png',
      isGuest: false,
    })
  })

  it('flags guests and tolerates missing fields', () => {
    const token = makeToken({ sub: 'guest-1', display_name: 'Guest', is_guest: true })
    const claims = decodeJwtClaims(token)
    expect(claims.isGuest).toBe(true)
    expect(claims.avatarUrl).toBeNull()
  })

  it('decodes UTF-8 display names', () => {
    const token = makeToken({ sub: 'u', display_name: 'Café ♠' })
    expect(decodeJwtClaims(token).displayName).toBe('Café ♠')
  })
})
