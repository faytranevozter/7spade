# Seven Spade — Mobile

React Native + Expo client for iOS and Android, with full feature parity to the
web SPA over the same `services/api` (HTTP) and `services/ws` (WebSocket)
backends.

## Setup

```bash
npm install
npm run ios       # iOS simulator
npm run android   # Android emulator
```

Set `EXPO_PUBLIC_API_URL` / `EXPO_PUBLIC_WS_URL` (see `.env.example`). On a
physical device, point them at your machine's LAN IP — `localhost` won't resolve
from the device.

## Scripts

| Command | Purpose |
|---|---|
| `npm run ios` / `npm run android` | Run on simulator / emulator |
| `npm test` | Jest unit tests for the ported pure logic |
| `npx tsc --noEmit` | Typecheck |

## Architecture

See [`docs/mobile.md`](../docs/mobile.md) for the full architecture: code reuse
from `web/`, the Expo Router layout, secure-storage auth, the deep-link OAuth
flow, and the backend changes it required.
