import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { SceneShell } from '../components/SceneShell'

/** Public site and operator identity used on legal pages (keep in sync with mobile store docs). */
const SITE_URL = 'https://spade.my.id'
const CONTACT_EMAIL = 'support@spade.my.id'
const OPERATOR_NAME = 'the individual operator of Seven Spade'
const GOVERNING_LAW = 'the laws of the Republic of Indonesia'
const LAST_UPDATED = '2026-07-19'
const PRIVACY_URL = `${SITE_URL}/privacy`
const TERMS_URL = `${SITE_URL}/terms`

function LegalProse({ children }: { children: ReactNode }) {
  return (
    <div className="mx-auto max-w-3xl space-y-6 text-sm leading-6 text-spade-gray-2 [&_h2]:mt-8 [&_h2]:text-lg [&_h2]:font-medium [&_h2]:text-spade-cream [&_h3]:mt-4 [&_h3]:text-sm [&_h3]:font-medium [&_h3]:text-spade-gold-light [&_li]:ml-5 [&_li]:list-disc [&_p]:text-spade-gray-2 [&_strong]:text-spade-cream [&_ul]:space-y-1.5">
      {children}
    </div>
  )
}

function LegalFooterLinks({ current }: { current: 'privacy' | 'terms' }) {
  return (
    <div className="mt-10 flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-spade-cream/10 pt-6 text-sm">
      {current === 'privacy' ? (
        <span className="text-spade-gray-3">Privacy Policy</span>
      ) : (
        <Link to="/privacy" className="text-spade-gold hover:text-spade-gold-light">
          Privacy Policy
        </Link>
      )}
      {current === 'terms' ? (
        <span className="text-spade-gray-3">Terms of Service</span>
      ) : (
        <Link to="/terms" className="text-spade-gold hover:text-spade-gold-light">
          Terms of Service
        </Link>
      )}
      <Link to="/register" className="text-spade-gold hover:text-spade-gold-light">
        Create account
      </Link>
      <Link to="/auth" className="text-spade-gold hover:text-spade-gold-light">
        Sign in
      </Link>
    </div>
  )
}

function OperatorBlock() {
  return (
    <p>
      <strong>Last updated:</strong> {LAST_UPDATED}
      <br />
      <strong>Product:</strong> Seven Spade ({SITE_URL})
      <br />
      <strong>Controller / operator:</strong> {OPERATOR_NAME}
      <br />
      <strong>Contact:</strong> {CONTACT_EMAIL}
    </p>
  )
}

export function PrivacyPolicyPage() {
  return (
    <SceneShell title="Privacy Policy" eyebrow="Legal">
      <LegalProse>
        <OperatorBlock />
        <p>
          This policy describes how Seven Spade (“the Service”), operated by {OPERATOR_NAME}, handles information when
          you use our multiplayer card game in a browser at {SITE_URL}. The same backend practices apply to our mobile
          app; the canonical public Privacy Policy URL is {PRIVACY_URL}.
        </p>

        <h2>1. Who we are</h2>
        <p>
          Seven Spade is a real-time multiplayer card game. The website and mobile clients talk to our game API and
          WebSocket services. The data controller for the production Service at {SITE_URL} is {OPERATOR_NAME},
          contactable at <strong>{CONTACT_EMAIL}</strong>. If you operate your own self-hosted backend deployment, you
          are the data controller for that deployment.
        </p>

        <h2>2. Information we collect</h2>
        <h3>2.1 Account &amp; profile (registered users)</h3>
        <p>When you create or link a registered account, we store in our database:</p>
        <ul>
          <li>Email address (optional for some OAuth paths such as Telegram)</li>
          <li>Password hash if you register with email (we do not store your password in plain text)</li>
          <li>Display name and unique username</li>
          <li>
            Linked OAuth provider records (provider name, provider user id, provider email if supplied, avatar URL) for
            providers you choose (e.g. GitHub, Google, Telegram)
          </li>
          <li>Email verification timestamp when you complete verification</li>
          <li>Account creation time</li>
        </ul>
        <p>
          Guests do not get a durable account row. Playing as guest issues a short-lived access token that carries a
          temporary id and display name only (no email, username, password, friends, stats, or refresh cookie).
        </p>

        <h3>2.2 Gameplay &amp; social data</h3>
        <ul>
          <li>
            Finished games and per-player results (display name, penalties, rank, win flag, optional user id, bot flag),
            including room name metadata where recorded
          </li>
          <li>
            Lifetime and seasonal stats for registered players (game counts, ratings, streaks, XP, achievements, and
            related aggregates)
          </li>
          <li>Per-game rating and XP event history for registered players</li>
          <li>
            Detailed result cards and full move/hand replay data for a limited number of recent finished games (older
            detail/replay rows are pruned on save)
          </li>
          <li>
            Room membership while a room exists (user id, display name, invite code, visibility, room settings, host
            kicks so a kicked player cannot rejoin that room)
          </li>
          <li>Friendships for registered users (pending, accepted, or blocked relationships between user ids)</li>
          <li>
            Live presence for registered users while connected (online and optional current room id), stored briefly in
            Redis with a short TTL and refreshed by heartbeat — not written for guests
          </li>
          <li>
            Live room snapshots in Redis while a room is active (roster, game board/hands, moves, timers, config), with
            TTL refresh while the room is used so abandoned rooms expire
          </li>
        </ul>
        <p>
          Public surfaces such as leaderboards and player profile pages may show display name, username, avatar (from a
          linked OAuth provider when present), and gameplay stats for registered players who appear there. Registered
          users may also be findable by username search for the friends feature.
        </p>

        <h3>2.3 Browser &amp; session data</h3>
        <ul>
          <li>
            Access JWT in browser <strong>sessionStorage</strong> for the current tab (includes subject id, display
            name, guest flag, and optional avatar URL claim)
          </li>
          <li>
            HttpOnly <strong>refresh_token</strong> cookie for registered sessions (about 30 days); the server stores
            only a hash of the refresh token in the database. This cookie is used solely to keep you signed in.
          </li>
          <li>Temporary invite code in sessionStorage when you open an invite link before joining</li>
          <li>
            Local preferences in browser localStorage (sound mute, card-animation speed, tutorial seen/skipped status)
          </li>
        </ul>
        <p>
          We do <strong>not</strong> require access to your contacts, camera, microphone, or location for core gameplay.
          We do not currently ship a third-party crash/analytics SDK in the web client.
        </p>

        <h3>2.4 Short-lived server cache (Redis)</h3>
        <p>In addition to presence and live rooms, the API may temporarily store:</p>
        <ul>
          <li>OAuth login state / PKCE verifier for the auth redirect flow</li>
          <li>Hashed one-time tokens for password reset and email verification (linked to user id, with TTL)</li>
          <li>Per-email or per-user rate-limit counters for recovery and similar actions</li>
        </ul>

        <h2>3. How we use information</h2>
        <ul>
          <li>Provide multiplayer gameplay, matchmaking, rooms, spectating, and replays</li>
          <li>Authenticate you and keep sessions secure (JWT access tokens, refresh rotation, logout)</li>
          <li>Maintain leaderboards, seasons, achievements, rating/XP, and history for registered players</li>
          <li>Enable friends, username search, invites, and account recovery (password reset / email verification)</li>
          <li>Improve reliability and prevent abuse (rate limits, room kicks, friendship blocks)</li>
          <li>Respond to your privacy and account requests (see Your rights)</li>
        </ul>

        <h2>4. Legal bases (where applicable)</h2>
        <p>
          If GDPR or similar laws apply: performance of a contract (providing the game), legitimate interests
          (security, anti-abuse, product improvement, public leaderboards as part of the game), and consent where
          required (e.g. optional marketing you add later).
        </p>

        <h2>5. Sharing</h2>
        <p>We share data with:</p>
        <ul>
          <li>
            <strong>Infrastructure providers</strong> that host our API, PostgreSQL database, Redis, and real-time
            WebSocket services
          </li>
          <li>
            <strong>OAuth providers</strong> you choose when starting or linking an account (you authorize them; we
            receive profile identifiers they return)
          </li>
          <li>
            <strong>Email delivery providers</strong> when SMTP is configured for password reset or verification
            messages (if SMTP is not configured, reset/verify links may only be logged server-side for development)
          </li>
          <li>
            <strong>Other players</strong> in the same room or on public leaderboards/profiles (display name, play
            state, and stats as shown in the product UI)
          </li>
          <li>
            <strong>Authorities</strong> when required by applicable law or to protect the Service and users
          </li>
        </ul>
        <p>We do not sell your personal information.</p>

        <h2>6. Retention</h2>
        <ul>
          <li>
            Registered account rows, friendships, lifetime/season stats, achievements, and game history summaries: while
            the account and related records exist, or until we complete a verified deletion request
          </li>
          <li>
            Guest identity: only inside the guest JWT until it expires (about 7 days) or you discard the tab/token;
            guests are not given a permanent user row
          </li>
          <li>
            Move-by-move replay and detailed face-down result data: limited to a recent window of finished games (on the
            order of the latest ~20); older detail is pruned when new games are saved. Basic game history rows may
            remain longer until account deletion or a broader purge
          </li>
          <li>Live room snapshots and presence keys in Redis: short TTL while active / connected</li>
          <li>OAuth state, password-reset, and email-verify token hashes in Redis: until consumed or TTL expiry</li>
          <li>
            Access JWT: until expiry (~7 days) or you clear sessionStorage / log out; refresh cookie and DB hash: until
            logout, rotation, password reset (all sessions revoked), or expiry (~30 days)
          </li>
          <li>Browser localStorage preferences: until you clear site data</li>
        </ul>

        <h2>7. Security</h2>
        <p>
          We use industry-standard transport security (HTTPS / WSS). Passwords are stored hashed. Refresh tokens are
          stored hashed server-side and sent to the browser only as an HttpOnly cookie (SameSite=Strict) on the web
          client. Access tokens live in sessionStorage for the tab. One-time email tokens are stored as hashes in Redis.
          No method of transmission or storage is 100% secure.
        </p>

        <h2>8. Children</h2>
        <p>
          The Service is not directed at children under 13 (or the minimum age in your region). Do not use the Service if
          you are under the applicable age without parental consent where required. If you believe we have collected
          data from a child in violation of applicable law, contact <strong>{CONTACT_EMAIL}</strong> and we will take
          reasonable steps to delete it.
        </p>

        <h2>9. Your choices</h2>
        <ul>
          <li>Play as guest (no durable account; limited features — no friends/stats account surface)</li>
          <li>Edit display name (registered users)</li>
          <li>
            Log out (clears the in-tab access token and instructs the server to clear the refresh cookie / revoke that
            refresh token)
          </li>
          <li>Mute sound, change motion speed, and dismiss or complete the tutorial (stored only in your browser)</li>
          <li>Exercise the rights described below, including account deletion by request</li>
        </ul>

        <h2>10. Your rights and account deletion</h2>
        <p>
          Depending on where you live, you may have rights to access, correct, delete, or restrict processing of your
          personal data, to object to certain processing, to data portability, and to lodge a complaint with a
          supervisory authority. You may also withdraw consent where processing is based on consent.
        </p>
        <p>
          <strong>How to make a request:</strong> email <strong>{CONTACT_EMAIL}</strong> from the address on your
          account (or include your username / user id and enough detail to verify ownership). State what you want (e.g.
          access copy, correction, deletion).
        </p>
        <p>
          <strong>Account deletion process (manual today):</strong> the product does not yet offer self-serve delete in
          the UI. When we verify a deletion request we will, within a reasonable period (target: within 30 days, sooner
          when feasible):
        </p>
        <ul>
          <li>Delete or anonymize your registered user account and linked OAuth provider rows</li>
          <li>Remove or anonymize friendships, stats, achievements, and session/refresh tokens tied to that account</li>
          <li>
            Anonymize or detach personal identifiers on historical game rows where feasible (e.g. clear user id while
            keeping non-identifying game outcome data needed for other players’ history)
          </li>
          <li>Confirm completion by email when the work is done, or explain any legal retention we must keep</li>
        </ul>
        <p>
          We may refuse or limit a request only where allowed by law (e.g. we cannot verify you control the account, or
          retention is required for security, fraud prevention, or legal obligations). Guest play leaves no durable
          account to delete beyond discarding your local token.
        </p>

        <h2>11. International transfers</h2>
        <p>
          If you access servers or infrastructure in another country, your data may be processed there. We rely on
          appropriate safeguards required by applicable law where transfers apply.
        </p>

        <h2>12. Governing law</h2>
        <p>
          This Privacy Policy is governed by {GOVERNING_LAW}, without regard to conflict-of-law rules, except where
          mandatory consumer or data-protection laws of your country of residence provide otherwise.
        </p>

        <h2>13. Changes</h2>
        <p>
          We may update this policy. We will revise the “Last updated” date and, for material changes, provide
          additional notice where required (for example via the Service or email when we have it).
        </p>

        <h2>14. Contact</h2>
        <p>
          Privacy and data-subject requests: <strong>{CONTACT_EMAIL}</strong>
          <br />
          Website: {SITE_URL}
          <br />
          Canonical policy URL: {PRIVACY_URL}
        </p>

        <LegalFooterLinks current="privacy" />
      </LegalProse>
    </SceneShell>
  )
}

export function TermsOfServicePage() {
  return (
    <SceneShell title="Terms of Service" eyebrow="Legal">
      <LegalProse>
        <OperatorBlock />
        <p>
          These Terms of Service (“Terms”) govern access to and use of Seven Spade at {SITE_URL} (the “Service”),
          operated by {OPERATOR_NAME}. By creating an account, playing as a guest, or otherwise using the Service, you
          agree to these Terms. Canonical URL: {TERMS_URL}.
        </p>

        <h2>1. The Service</h2>
        <p>
          Seven Spade lets players join rooms, play card games (including with bots), spectate, view history and
          leaderboards, and use related social features such as friends when available. Features may change, expand, or
          be limited by account type (guest vs registered).
        </p>

        <h2>2. Eligibility</h2>
        <p>
          You must be old enough to use the Service under the laws of your region (and at least 13 where that is the
          minimum). If you use the Service on behalf of an organization, you represent that you have authority to bind
          it to these Terms.
        </p>

        <h2>3. Accounts</h2>
        <ul>
          <li>You may play as a guest or register with email/password or supported OAuth providers.</li>
          <li>You are responsible for activity under your account and for keeping credentials secure.</li>
          <li>Provide accurate information when registering; do not impersonate others.</li>
          <li>We may suspend or terminate accounts that violate these Terms or abuse the Service.</li>
          <li>
            To request account deletion, email <strong>{CONTACT_EMAIL}</strong> (see the Privacy Policy for the process).
          </li>
        </ul>

        <h2>4. Acceptable use</h2>
        <p>You agree not to:</p>
        <ul>
          <li>Cheat, exploit bugs, automate play in a way that harms fair multiplayer, or disrupt rooms</li>
          <li>Harass other players, spam, or abuse chat/emotes or social features</li>
          <li>Attempt unauthorized access to accounts, APIs, WebSockets, or infrastructure</li>
          <li>Reverse engineer or scrape the Service except as allowed by law</li>
          <li>Use the Service for unlawful purposes</li>
        </ul>
        <p>
          Room hosts and the Service may remove players (e.g. kick) and enforce rate limits or other anti-abuse
          measures.
        </p>

        <h2>5. Gameplay, ratings, and content</h2>
        <p>
          Game outcomes, ratings, XP, achievements, and leaderboards are provided as part of the entertainment
          experience and may be adjusted for fairness, bugs, or abuse. User-generated content (such as display names)
          must not be offensive or infringing; we may moderate or remove it.
        </p>

        <h2>6. Intellectual property</h2>
        <p>
          The Service, branding, UI, and game software are owned by {OPERATOR_NAME} or their licensors. These Terms do
          not grant you ownership of the Service. You retain rights to content you submit, and grant us a limited
          license to host and display it as needed to run the Service.
        </p>

        <h2>7. Privacy</h2>
        <p>
          How we handle personal data is described in our{' '}
          <Link to="/privacy" className="text-spade-gold hover:text-spade-gold-light">
            Privacy Policy
          </Link>
          . By using the Service you acknowledge that policy.
        </p>

        <h2>8. Availability and changes</h2>
        <p>
          We aim for reliable uptime but do not guarantee uninterrupted access. We may modify, suspend, or discontinue
          features (including custom rooms, seasons, or integrations) at any time.
        </p>

        <h2>9. Disclaimers</h2>
        <p>
          THE SERVICE IS PROVIDED “AS IS” AND “AS AVAILABLE” WITHOUT WARRANTIES OF ANY KIND, EXPRESS OR IMPLIED,
          INCLUDING MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT, TO THE MAXIMUM EXTENT
          PERMITTED BY LAW. NOTHING IN THESE TERMS LIMITS RIGHTS THAT CANNOT BE WAIVED UNDER MANDATORY CONSUMER LAW.
        </p>

        <h2>10. Limitation of liability</h2>
        <p>
          To the maximum extent permitted by law, {OPERATOR_NAME} is not liable for indirect, incidental, special,
          consequential, or punitive damages, or loss of data, profits, or goodwill, arising from your use of the
          Service. Where liability cannot be excluded, it is limited to the greater of fees you paid us for the Service
          in the three months before the claim (if any) or a nominal amount required by law.
        </p>

        <h2>11. Termination</h2>
        <p>
          You may stop using the Service at any time (including by logging out) and may request account deletion as
          described in the Privacy Policy. We may suspend or end access if you breach these Terms or if we shut down or
          restructure the Service.
        </p>

        <h2>12. Governing law and disputes</h2>
        <p>
          These Terms are governed by {GOVERNING_LAW}, without regard to conflict-of-law rules. Courts in Indonesia
          have non-exclusive jurisdiction, except where mandatory law gives you the right to bring claims in your place
          of residence.
        </p>

        <h2>13. Changes to these Terms</h2>
        <p>
          We may update these Terms. We will revise the “Last updated” date and, for material changes, provide
          additional notice where required. Continued use after changes become effective constitutes acceptance, except
          where mandatory law requires a different process.
        </p>

        <h2>14. Contact</h2>
        <p>
          Questions about these Terms: <strong>{CONTACT_EMAIL}</strong>
          <br />
          Website: {SITE_URL}
        </p>

        <LegalFooterLinks current="terms" />
      </LegalProse>
    </SceneShell>
  )
}
