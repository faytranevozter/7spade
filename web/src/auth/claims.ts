// Decodes the (unverified) JWT payload the app already holds, so the client can
// read its own identity for UI gating. The token is still verified server-side
// on every request; this is purely for presentation (e.g. hide the friends UI
// for guests, identify "me").
export type JwtClaims = {
  userId: string | null
  displayName: string | null
  avatarUrl: string | null
  isGuest: boolean
}

export function decodeJwtClaims(token: string | null): JwtClaims {
  const empty: JwtClaims = { userId: null, displayName: null, avatarUrl: null, isGuest: false }
  if (!token) return empty
  const parts = token.split('.')
  if (parts.length < 2) return empty
  try {
    const payload = JSON.parse(
      atob(parts[1].replace(/-/g, '+').replace(/_/g, '/')),
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
