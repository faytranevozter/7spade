import { Link } from 'react-router'
import { Button } from '../components/Button'

const SITE_URL = 'https://spade.my.id'
const CONTACT_EMAIL = 'support@spade.my.id'

export function LandingPage() {
  return (
    <section className="min-h-svh bg-spade-bg px-4 py-10 sm:px-6 sm:py-14">
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-10">
        <header className="flex flex-col items-center gap-4 text-center">
          <img src="/logo.png" alt="Seven Spade" className="size-16 sm:size-20" />
          <div className="grid gap-2">
            <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-spade-gold-light">
              Free browser card game
            </p>
            <h1 className="text-3xl font-bold tracking-tight text-spade-gold-light sm:text-4xl">
              Seven Spade
            </h1>
            <p className="mx-auto max-w-xl text-base leading-relaxed text-spade-gray-2 sm:text-lg">
              Seven Spade is a free, real-time multiplayer card game based on the classic game of 7s.
              Build suit sequences, avoid penalty cards, and compete for the lowest score with friends or bots.
            </p>
          </div>
          <div className="flex flex-wrap items-center justify-center gap-3 pt-1">
            <Link to="/auth">
              <Button type="button" className="min-w-[9rem] px-6 py-3">
                Sign in / Play
              </Button>
            </Link>
            <Link to="/register">
              <Button type="button" variant="secondary" className="min-w-[9rem] px-6 py-3">
                Create account
              </Button>
            </Link>
          </div>
        </header>

        <div className="rounded-spade-xl border border-spade-cream/10 bg-[#102316] p-6 shadow-spade-card sm:p-8">
          <h2 className="text-xl font-medium text-spade-cream">What is Seven Spade?</h2>
          <p className="mt-3 text-sm leading-6 text-spade-gray-2 sm:text-base">
            Players build suit sequences on the table starting from the sevens and extending outward
            (6s and 8s, then 5s and 9s, and so on). When you cannot play a legal card, you must place
            one face-down as a penalty. Aces never extend a run—they only close a suit high or low.
            When the round ends, the player (or team) with the <strong className="text-spade-cream">lowest
            penalty</strong> wins. Seven Spade supports casual rooms, practice with bots, friends,
            live spectating, and ranked play with a leaderboard.
          </p>

          <h2 className="mt-8 text-xl font-medium text-spade-cream">How it works</h2>
          <ul className="mt-3 list-disc space-y-2 pl-5 text-sm leading-6 text-spade-gray-2 sm:text-base">
            <li>Join a room or start practice; empty seats can be filled with bots.</li>
            <li>Cards are dealt; the game opens around the sevens, typically starting with 7♠.</li>
            <li>On your turn, play a card that extends an open suit sequence, or go face-down if you cannot.</li>
            <li>Aces close a suit (high after King, or low after 2); the first Ace method locks for the table.</li>
            <li>When no one can play or hands empty out, penalties are tallied—lowest score ranks best.</li>
          </ul>

        </div>

        <section aria-labelledby="ways-to-play-heading">
          <div className="mb-5 flex items-end justify-between gap-4">
            <div>
              <p className="font-mono text-[10px] uppercase tracking-[0.2em] text-spade-gold">More than one table</p>
              <h2 id="ways-to-play-heading" className="mt-1 text-2xl font-medium text-spade-cream">
                Play, progress, reconnect
              </h2>
            </div>
            <span className="hidden font-mono text-4xl text-spade-cream/10 sm:block">♠</span>
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <article className="rounded-spade-lg border border-spade-cream/10 bg-[#0d1d12] p-5">
              <p className="font-mono text-[10px] uppercase tracking-[0.18em] text-spade-gold-light">Play your way</p>
              <h3 className="mt-2 text-lg font-medium text-spade-cream">A table for every group</h3>
              <p className="mt-3 text-sm leading-6 text-spade-gray-2">
                Join classic games or create custom rooms for 2–8 players with teams, multiple decks, and alternate
                scoring. Practice with bots or climb the ranked leaderboard.
              </p>
            </article>

            <article className="rounded-spade-lg border border-spade-cream/10 bg-[#0d1d12] p-5">
              <p className="font-mono text-[10px] uppercase tracking-[0.18em] text-spade-gold-light">Earn and unlock</p>
              <h3 className="mt-2 text-lg font-medium text-spade-cream">Make every game count</h3>
              <p className="mt-3 text-sm leading-6 text-spade-gray-2">
                Gain XP, complete achievements and events, claim daily rewards, and unlock skins that personalize your
                place at the table.
              </p>
            </article>

            <article className="rounded-spade-lg border border-spade-cream/10 bg-[#0d1d12] p-5">
              <p className="font-mono text-[10px] uppercase tracking-[0.18em] text-spade-gold-light">Stay connected</p>
              <h3 className="mt-2 text-lg font-medium text-spade-cream">Follow every match</h3>
              <p className="mt-3 text-sm leading-6 text-spade-gray-2">
                Add friends, explore player profiles, spectate live rooms, and revisit finished games through history
                and move-by-move replays.
              </p>
            </article>
          </div>
        </section>

        <footer className="grid gap-4 border-t border-spade-cream/10 pt-6 text-center text-sm text-spade-gray-3">
          <nav aria-label="Legal and account" className="flex flex-wrap items-center justify-center gap-x-4 gap-y-2">
            <a href="/privacy" className="font-medium text-spade-gold hover:text-spade-gold-light">
              Privacy Policy
            </a>
            <a href="/terms" className="font-medium text-spade-gold hover:text-spade-gold-light">
              Terms of Service
            </a>
            <Link to="/auth" className="font-medium text-spade-gold hover:text-spade-gold-light">
              Sign in
            </Link>
            <Link to="/register" className="font-medium text-spade-gold hover:text-spade-gold-light">
              Create account
            </Link>
          </nav>
          <p>
            <span className="text-spade-cream">Seven Spade</span>
            {' · '}
            <a href={SITE_URL} className="text-spade-gold hover:text-spade-gold-light">
              {SITE_URL.replace(/^https:\/\//, '')}
            </a>
            {' · '}
            <a href={`mailto:${CONTACT_EMAIL}`} className="text-spade-gold hover:text-spade-gold-light">
              {CONTACT_EMAIL}
            </a>
          </p>
        </footer>
      </div>
    </section>
  )
}
