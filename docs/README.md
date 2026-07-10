# Seven Spade — Documentation

A real-time multiplayer card game built with Go and React.

## Contents

| Document | Description |
|---|---|
| [Game Rules](./game-rules.md) | Classic rules + custom modes |
| [Architecture](./architecture.md) | System design, scaling, storage |
| [API Reference](./api.md) | HTTP API endpoints |
| [OpenAPI](./openapi.yaml) | Machine-readable API schema |
| [WebSocket Protocol](./websocket.md) | Real-time game message protocol |
| [Development Guide](./development.md) | Local setup, environment variables, and project structure |
| [Deployment Guide](./deployment/) | Production deployment, reverse proxy, TLS, backups, CI/CD |
| [Multi-Provider OAuth](./multi-provider-oauth.md) | Google / GitHub / Telegram OAuth + OIDC flow |
| [Roadmap](./roadmap.md) | Feature backlog and implementation status |

## Specs

Detailed feature specifications live under [`specs/`](./specs/).

| Spec | Status |
|---|---|
| [Player Stats & Leaderboard](./specs/stats-and-leaderboard.md) | Implemented |
| [Seasons & Skill Rating (ELO)](./specs/seasons-and-elo.md) | Implemented |
| [Achievements & Badges](./specs/achievements.md) | Implemented |
| [In-Game Emotes / Quick-Chat](./specs/emotes.md) | Implemented |
| [Player Avatars End-to-End](./specs/player-avatars.md) | Implemented |
| [Sound Effects & Mute](./specs/sound-effects.md) | Implemented |
| [Spectator Mode](./specs/spectator-mode.md) | Implemented |
| [Friends & Presence](./specs/friends-and-presence.md) | Implemented |
| [Bot Difficulty Levels](./specs/bot-difficulty.md) | Implemented |
| [Practice Mode](./specs/practice-mode.md) | Implemented |
| [Custom Game Modes](./specs/custom-game-modes.md) | Implemented |
| [Password Reset & Email Verification](./specs/password-reset-and-email-verification.md) | Implemented |

Related: [XP feature plan](./xp-feature-plan.md) (shipped — XP/levels on `game_over` and stats).
