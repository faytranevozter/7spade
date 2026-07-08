# Spec: Password Reset & Email Verification

Status: Implemented
Owner: —
Related: [HTTP API](../api.md) · [Architecture](../architecture.md)

## 1. Overview

Account recovery and email verification for email/password users:

- **Password reset** — request a reset link by email, set a new password with a
  single-use, time-limited token, and invalidate all existing sessions.
- **Email verification** — a verification link is emailed on registration;
  verification is **soft** (unverified users can still play) and surfaced via a
  dismissible banner.

### Non-goals

- Blocking gameplay or features for unverified users.

## 2. Tokens & storage

- Tokens are cryptographically random (`crypto/rand`, 32 bytes, URL-safe), the
  same generator as refresh tokens.
- Only the **SHA-256 hash** of a token is stored, as a Redis key; the raw token
  travels only in the emailed link. A Redis dump therefore can't be replayed.
- Single-use: consumption atomically reads and deletes the key (Redis
  `TxPipeline` GET+DEL).
- TTLs: password reset **15 minutes**, email verification **24 hours**.
- Redis keys: `password_reset:{sha256}`, `email_verify:{sha256}` → user id.

## 3. Email sending

- `internal/email` defines a `Sender` interface
  (`SendPasswordReset`, `SendVerification`).
- `SMTPSender` (stdlib `net/smtp`, `html/template`) is used when `SMTP_HOST` is
  set; otherwise `LogSender` logs the link to the API console (dev default — no
  SMTP required).
- `NewFromConfig` selects the implementation. Templates are plain HTML, no
  external dependencies.
- Config: `SMTP_HOST`, `SMTP_PORT` (default 587), `SMTP_USER`, `SMTP_PASS`,
  `SMTP_FROM`. `docker-compose.yml` ships an optional, commented **Mailpit**
  service for a local inbox.

## 4. Endpoints

| Endpoint | Auth | Notes |
|---|---|---|
| `POST /auth/forgot-password` | public | Always `200` (no email enumeration); emails a link only when a password account exists. |
| `POST /auth/reset-password` | public | Consumes token, sets bcrypt hash, **revokes all refresh tokens**. |
| `POST /auth/verify-email` | public | Consumes token, stamps `users.email_verified_at`. |
| `POST /auth/resend-verification` | bearer JWT | No-op if verified/guest; always `204`. |

`GET /me` gains `email_verified` (derived from `email_verified_at != null`).
Registration fires a verification email (non-fatal on send failure).

### Rate limiting

Per-email fixed-window limits guard the email-sending endpoints (Redis
`INCR`+`EXPIRE`, keys `rate:{scope}:{email}`):

- `forgot-password`: max **3 / hour** per email (scope `pwreset`).
- `resend-verification`: max **5 / hour** per email (scope `verify`).

Limits are checked before minting/sending, so a hit neither mints a token nor
sends mail — but the endpoints still return their normal `200`/`204` so they
remain enumeration-safe. Registration's initial verification email is not
limited (it happens once per signup). The limiter fails **open** on a Redis
error so a cache blip can't lock users out of recovery.

## 5. Data model

Migration `014_email_verification.sql`:

```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMP NULL;
```

(Also: migration `013_bot_difficulty_repair.sql` is unrelated — a constraint
repair from the bot-difficulty work.)

## 6. Single link format & clients

One link format is emailed (web app URLs from `FRONTEND_URL`):
`/{reset-password,verify-email}?token=…`.

- **Web** — pages at `/forgot-password`, `/reset-password`, `/verify-email`; a
  "Forgot password?" link on the login/auth screens; a verification banner in
  the app shell.
- **Native clients** — may handle the same token through deep links such as
  `sevenspade://reset?token=…` and `sevenspade://verify?token=…`.

## 7. Rate limiting

The issue's per-email limits are enforced (see §4 "Rate limiting"): 3 reset/hr
and 5 verify/hr, via a Redis `INCR`+`EXPIRE` fixed window keyed by
`rate:{scope}:{email}`. The check runs before token minting / email send and is
enumeration-safe (responses are unchanged). It fails open on Redis errors.

## 8. Testing

- **API** — `email` (sender selection, template rendering); `cache` token store
  with **miniredis** (single-use, expiry, unknown token) and the rate limiter
  (limit enforced, per-email/scope isolation, window reset); repository
  (`UpdatePasswordHash`, `MarkEmailVerified`, `RevokeAllRefreshTokensForUser`);
  handler flow (forgot always-200, forgot→reset→reused-token, verify
  single-use, resend guards, forgot rate-limit stops sending after 3) using
  miniredis + sqlmock + a fake sender.
- **Web** — forgot/reset/verify page flows (vitest).
