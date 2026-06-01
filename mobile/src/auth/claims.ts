// Decodes the (unverified) JWT payload the app already holds, so the client can
// read its own identity for UI gating. The token is still verified server-side
// on every request; this is purely for presentation (e.g. hide the friends UI
// for guests, identify "me").
//
// Ported from web/src/auth/claims.ts. The web version relies on the browser's
// `atob`; React Native's Hermes engine doesn't reliably expose it, so we decode
// base64url ourselves.
export type JwtClaims = {
  userId: string | null
  displayName: string | null
  avatarUrl: string | null
  isGuest: boolean
}

const BASE64_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'

// decodeBase64 turns a standard base64 string into its decoded bytes, then into
// a UTF-8 string. Self-contained so it works on Hermes without `atob`.
function decodeBase64(input: string): string {
  const clean = input.replace(/[^A-Za-z0-9+/]/g, '')
  const bytes: number[] = []
  let buffer = 0
  let bits = 0
  for (const char of clean) {
    const value = BASE64_CHARS.indexOf(char)
    if (value === -1) continue
    buffer = (buffer << 6) | value
    bits += 6
    if (bits >= 8) {
      bits -= 8
      bytes.push((buffer >> bits) & 0xff)
    }
  }
  return utf8Decode(bytes)
}

// utf8Decode reconstructs a JS string from a UTF-8 byte sequence.
function utf8Decode(bytes: number[]): string {
  let result = ''
  let i = 0
  while (i < bytes.length) {
    const byte1 = bytes[i++]
    if (byte1 < 0x80) {
      result += String.fromCharCode(byte1)
    } else if (byte1 >= 0xc0 && byte1 < 0xe0) {
      const byte2 = bytes[i++] & 0x3f
      result += String.fromCharCode(((byte1 & 0x1f) << 6) | byte2)
    } else if (byte1 >= 0xe0 && byte1 < 0xf0) {
      const byte2 = bytes[i++] & 0x3f
      const byte3 = bytes[i++] & 0x3f
      result += String.fromCharCode(((byte1 & 0x0f) << 12) | (byte2 << 6) | byte3)
    } else {
      const byte2 = bytes[i++] & 0x3f
      const byte3 = bytes[i++] & 0x3f
      const byte4 = bytes[i++] & 0x3f
      const codePoint = ((byte1 & 0x07) << 18) | (byte2 << 12) | (byte3 << 6) | byte4
      result += String.fromCodePoint(codePoint)
    }
  }
  return result
}

export function decodeJwtClaims(token: string | null): JwtClaims {
  const empty: JwtClaims = { userId: null, displayName: null, avatarUrl: null, isGuest: false }
  if (!token) return empty
  const parts = token.split('.')
  if (parts.length < 2) return empty
  try {
    const payload = JSON.parse(
      decodeBase64(parts[1].replace(/-/g, '+').replace(/_/g, '/')),
    ) as { sub?: string; display_name?: string; avatar_url?: string; is_guest?: boolean }
    return {
      userId: payload.sub ?? null,
      displayName: payload.display_name ?? null,
      avatarUrl: payload.avatar_url ?? null,
      isGuest: Boolean(payload.is_guest),
    }
  } catch {
    return empty
  }
}
