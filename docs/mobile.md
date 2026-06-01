# Mobile App

The mobile app (`mobile/`) is a React Native + Expo client for iOS and Android,
with full feature parity to the web SPA. It talks to the same `services/api`
(HTTP) and `services/ws` (WebSocket) backends over the identical wire protocol.

## Stack

| Concern | Choice |
|---|---|
| Framework | React Native via **Expo** (SDK 54) |
| Navigation | **Expo Router** (file-based, typed routes) |
| Styling | **NativeWind** (Tailwind for RN); tokens mirror the web `@theme` |
| Secure storage | **expo-secure-store** (Keychain / Keystore) |
| OAuth | **expo-auth-session** + **expo-web-browser** (deep-link redirect) |
| Feedback cues | **Vibration** haptics (native port of the web Web-Audio cues) |
| Tests | **jest** + **ts-jest** for the ported pure logic |

## Code reuse from `web/`

The web app cleanly separates pure logic from DOM UI. The mobile app reuses the
pure logic and rebuilds only the UI:

**Ported (logic identical to web):**

- `src/types.ts` — wire/domain types (`Card`, `Player`, `BoardRow`, ...)
- `src/game/cards.ts` — suit/rank conversion, board column math, initials
- `src/game/emotes.ts`, `src/game/achievements.ts` — catalogs
- `src/auth/claims.ts` — JWT claim decode (with a Hermes-safe base64url decoder
  since RN lacks `atob`)
- `src/api/*` — fetch wrappers (env-driven base URL via `expo-constants`)
- `src/hooks/useGameSocket.ts` / `useSpectatorSocket.ts` — the message reducer,
  `buildBoardRows`, and sound-cue derivation are ported unchanged; only the
  socket transport and reconnect logic differ

**Rebuilt with React Native primitives:**

- `src/components/*` — `Button`, `Badge`, `CardFace`, `GameBoard`, `Modal`,
  `EmotePicker`, `ScoreTable`, etc. (View / Text / Pressable + NativeWind)
- `app/**` — every screen, as Expo Router routes

## Routing

Expo Router file-based routes under `mobile/app/`:

- `app/_layout.tsx` — root: `AuthProvider`, safe-area + gesture roots, and a
  navigator-level auth gate (replaces the web app's per-page inline guards)
- `app/(auth)/` — signed-out group: `index` (guest + login + OAuth), `register`,
  `auth/callback` (OAuth deep-link landing)
- `app/(app)/` — authenticated group: `lobby`, `room/[id]` (waiting room),
  `game/[id]`, `spectate/[id]`, `history`, `leaderboard`, `friends`,
  `profile/[id]`

## What differs from web (and why)

1. **Auth persistence.** Web keeps the access JWT in `sessionStorage` and the
   refresh token in an HttpOnly cookie. Native has neither, so both tokens live
   in `expo-secure-store`. On cold start the `AuthProvider` hydrates from
   storage and, if the access JWT is expired but a refresh token exists, calls
   `POST /refresh` before marking the session ready — so users stay signed in
   across launches.

2. **OAuth deep link.** Native can't rely on browser redirects + cookies. The
   `useOAuth` hook builds a `sevenspade://auth/callback` redirect URI, requests
   the provider authorize URL from the API (which still holds the PKCE verifier
   in Redis), opens it via `WebBrowser.openAuthSessionAsync`, and exchanges the
   returned `code`/`state` for the app JWT + refresh token.

3. **Realtime resilience.** Mobile sockets get suspended on backgrounding and
   drop on flaky networks, so `useGameSocket`/`useSpectatorSocket` add capped
   exponential-backoff auto-reconnect and reconnect-on-foreground (via
   `AppState`). The web app only reconnects manually.

4. **Sound.** The web app synthesises cues with the Web Audio API. The native
   port keeps the same `audioManager` interface (`play`/`mute`/`unlock`/
   `subscribe`) but renders cues as short `Vibration` haptics, so no audio
   assets are shipped. Swapping in `expo-audio` samples later is a localised
   change behind the same interface.

## Backend changes this required

The mobile app needed two small, additive changes in `services/api` — no
WebSocket-server changes (its `CheckOrigin` already allows all and auth is via a
query-param JWT, both transport-agnostic).

1. **Token-in-body refresh / logout.** `POST /refresh` and
   `DELETE /auth/logout` now also accept `{ "refresh_token": "..." }` in the
   body when no cookie is present, and `/register`, `/login`, `/refresh`, and the
   OAuth callback echo a rotated `refresh_token` in the body for native callers.
   Web behaviour is unchanged (cookie path takes precedence).

2. **OAuth `redirect_uri` passthrough.** `GET /auth/:provider/url` accepts an
   optional `redirect_uri` query param (allowlisted to `sevenspade://` / `exp://`
   to prevent open-redirect abuse), stores it with the PKCE state in Redis, and
   replays it verbatim in the token exchange. Web omits it and falls back to the
   provider's configured default.

## Config

`mobile/app.config.ts` reads backend URLs from env (`EXPO_PUBLIC_API_URL`,
`EXPO_PUBLIC_WS_URL`) into `extra`, surfaced via `src/config.ts`. On a physical
device, point these at your machine's LAN IP (localhost won't resolve from the
device). The deep-link scheme is `sevenspade`.

## Commands

```bash
cd mobile && npm install        # install deps
cd mobile && npm run ios        # run on iOS simulator
cd mobile && npm run android    # run on Android emulator
cd mobile && npm test           # jest unit tests (ported logic)
cd mobile && npx tsc --noEmit   # typecheck
```

## Status

Implemented: scaffold, shared-logic port, auth (guest / email / OAuth deep-link),
lobby, waiting room, live game (board, hand, Ace-close, turn timer, emotes),
results + rematch, history, leaderboard, profile, friends, and spectator.

Not yet done (future work): EAS Build profiles + store assets (icon/splash),
provider redirect-URI registration for production OAuth, push notifications, and
extraction of the shared logic into a `packages/shared` workspace consumed by
both `web/` and `mobile/` (currently copied).
