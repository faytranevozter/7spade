# Spec: Player Avatars End-to-End

Status: Implemented
Owner: —
Related: [Architecture](../architecture.md) · [HTTP API](../api.md) · [WebSocket Protocol](../websocket.md) · [Multi-Provider OAuth](../multi-provider-oauth.md)

## 1. Overview

OAuth sign-in already captures each provider's avatar URL into
`user_providers.avatar_url`, but it is never displayed. This feature surfaces
that avatar across every player-facing surface — the live game table, the
waiting room, the leaderboard, and public profiles — with a graceful initials
fallback for players who have none.

### Goals

- Show a player's OAuth avatar wherever they're represented (game seats, lobby
  slots, leaderboard rows, profile header).
- Fall back to the existing tone + initials circle when no avatar exists.
- Keep one consistent, deterministic avatar per user.

### Non-goals

- Avatar upload / custom avatars (only provider-sourced images).
- Avatars for guests or bots (they have no durable identity / no provider row).
- A canonical avatar column on `users` (avatars stay in `user_providers`).
- Real-time avatar refresh (a provider photo change appears after the next
  login/refresh; see §7).

## 2. Avatar Source & Selection

Avatars live **only** in `user_providers.avatar_url` (nullable, one row per
linked provider). A user may link several providers, so a deterministic
selection rule picks one: **provider precedence** (google > github > telegram),
newest link as tiebreak.

```sql
LEFT JOIN LATERAL (
  SELECT up.avatar_url
  FROM user_providers up
  WHERE up.user_id = u.id AND up.avatar_url IS NOT NULL
  ORDER BY CASE up.provider
             WHEN 'google'   THEN 0
             WHEN 'github'   THEN 1
             WHEN 'telegram' THEN 2
             ELSE 3
           END,
           up.created_at DESC
  LIMIT 1
) av ON true
```

A `LATERAL ... LIMIT 1` (not a plain `JOIN`) keeps exactly one row per user, so
multi-provider users don't multiply leaderboard rows. The avatar is always
nullable end-to-end (`*string` / optional), since guests, bots, and
email/password-only users have none.

## 3. Two Delivery Paths

The avatar reaches the client two ways, because two layers own player identity:

- **Static surfaces** (leaderboard, public profile, my-stats) read the avatar
  directly from the DB in the stats queries that already join `users`.
- **Live surfaces** (in-game seats, waiting-room slots) get identity from the
  WebSocket service, which only sees the JWT. So the avatar is denormalized into
  the JWT as a claim (mirroring `display_name`) and threaded through the WS
  payloads.

## 4. JWT Claim

`auth.Claims` gains `AvatarURL string \`json:"avatar_url,omitempty"\``.
`GenerateUserToken` takes an `avatarURL` argument; `GenerateGuestToken` is
unchanged (guests never have one). Call sites:

- **OAuth callback** — passes `profile.AvatarURL`, already fetched from the
  provider; zero extra queries.
- **Register / Login / Refresh** — resolve the avatar via a new repository
  helper `GetUserAvatar(db, userID) (*string, error)` (the §2 query keyed by
  `user_id`). Email/password-only users resolve to `nil`.

The claim lets the WS service and the `/stats` "no games yet" fallback obtain an
avatar without an extra lookup.

## 5. Server Design

### API (`services/api`)

- `auth/jwt.go` — `Claims.AvatarURL`; `GenerateUserToken(userID, displayName,
  avatarURL, secret)`.
- `repository/user.go` — `GetUserAvatar(db, userID)` using the LATERAL query.
- `repository/stats.go` — `UserStats.AvatarURL *string` and
  `LeaderboardEntry.AvatarURL *string`; `GetLeaderboard` and `GetUserStats`
  SELECTs extended with the LATERAL join + scan targets. The rank subquery is
  unchanged.
- `handler/stats.go` — unchanged response shapes (structs already serialize the
  new field); the `/stats` zero-games fallback populates `AvatarURL` from
  `claims.AvatarURL`.

### WS (`services/ws`)

- `tokenClaims.AvatarURL` read from the JWT; `player.avatar` field set in
  `addLobbyPlayerLocked`.
- `avatar_url` added to the lobby payload (both connected/disconnected
  branches), the `opponents` array in `state_update`, and the `results` array in
  `game_over`.
- Snapshot persistence: `Avatar` added to `persistedPlayer` and
  `store.PersistedPlayer` plus the four copy sites, so avatars survive a WS
  restart / lazy rehydrate.

## 6. Frontend (`web`)

- **`Avatar` component** — renders an `<img>` when a URL is present (with
  `onError` → initials fallback, `referrerPolicy="no-referrer"`,
  `loading="lazy"`), else the existing tone + initials circle. Reused by the
  game seats, lobby slots, leaderboard, and profile.
- **Types / DTOs** — optional `avatarUrl` on `Player` and `LobbyPlayer`;
  `avatar_url` on `StateUpdateMessage.opponents`, `GameOverMessage.results`,
  `LobbyStateMessage.players`, `LeaderboardEntryDto`, `UserStatsDto`.
- **`useGameSocket`** — thread `avatar_url` through `buildPlayers` (opponents),
  the game-over mapper, and the lobby mapper. The local "You" seat decodes its
  own `avatar_url` from the JWT (extended `decodeJwt*` helper).
- **Render sites** — `OpponentCard` (game), the waiting-room slot, leaderboard
  rows, and the profile header swap their initials circle for `Avatar`. The
  emote bubble overlay on seats is preserved.

## 7. Edge Cases

- **No avatar**: guests, bots, email/password-only users, and image load
  failures all fall back to tone + initials.
- **Multiple providers**: provider-precedence selection picks one
  deterministically; no row multiplication.
- **Staleness**: JWT-borne avatars are fixed for the token TTL (7 days); a
  provider photo change appears after the next login/refresh. DB-read surfaces
  (leaderboard/profile) are always current.
- **Reconnect / WS restart**: the avatar is persisted in the room snapshot, so
  rehydrated rooms keep it.
- **Privacy**: third-party image URLs load cross-origin; `referrerPolicy=
  "no-referrer"` avoids leaking the app URL. Proxying/caching is future work.

## 8. Testing

- **API**: `GetUserAvatar` precedence + null; `GetLeaderboard`/`GetUserStats`
  return the avatar without row multiplication; `/stats` fallback carries the
  claim avatar.
- **WS**: lobby / `state_update` / `game_over` payloads include `avatar_url`;
  snapshot round-trips it.
- **Web**: `Avatar` renders img vs initials and falls back on `onError`;
  leaderboard/profile render avatars from DTOs; socket mocks updated.
- Run `make -C services/api test`, `make -C services/ws test`, and
  `cd web && npm test && npm run lint && npm run build`.

## 9. Open Questions / Future Work

- **Avatar proxy / cache** — serve provider images through the app to remove the
  cross-origin/referrer concern and tolerate provider link rot.
- **Custom avatar upload** — let any user (incl. email/password) set an avatar,
  which would justify a canonical `users.avatar_url`.
- **Refresh on change** — re-issue or refetch avatars more eagerly than the
  token TTL.
