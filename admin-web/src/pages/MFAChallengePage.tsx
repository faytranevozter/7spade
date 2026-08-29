import { useState, type FormEvent } from 'react'
import { AdminBrand } from '../components/AdminBrand'
import { Notice } from '../components/Feedback'
import { useAuth } from '../hooks/useAuth'

export function MFAChallengePage() {
  const { completeMFA, error } = useAuth()
  const [submitting, setSubmitting] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    const data = new FormData(event.currentTarget)
    await completeMFA(String(data.get('code')))
    setSubmitting(false)
  }

  return (
    <main className="grid min-h-screen place-items-center bg-[#0a0d12] bg-[radial-gradient(circle_at_80%_-20%,#163538_0,transparent_34%),linear-gradient(#ffffff08_1px,transparent_1px),linear-gradient(90deg,#ffffff08_1px,transparent_1px)] bg-size-[auto,48px_48px,48px_48px] p-6">
      <section className="w-full max-w-115 border border-t-[3px] border-[#2a3844] border-t-[#4dd0b5] bg-[#10151cdd] p-10 shadow-[0_30px_90px_#000000aa]">
        <AdminBrand subtitle="Restricted operations console" />
        <p className="mt-6 mb-2 font-mono text-[11px] font-bold tracking-[0.15em] text-[#4dd0b5]">
          SECOND FACTOR REQUIRED
        </p>
        <h1 className="my-2.5 text-3xl leading-none font-bold tracking-[-0.045em] text-white sm:text-4xl md:text-5xl">
          Verify your identity
        </h1>
        <p className="leading-[1.6] text-[#99a5b3]">
          Enter the six-digit code from your authenticator or a recovery code.
        </p>
        <form onSubmit={submit} className="mt-7.5 grid gap-4.5">
          <label className="grid gap-2 text-sm text-[#aeb8c4]">
            Authentication code
            <input
              name="code"
              inputMode="numeric"
              autoComplete="one-time-code"
              required
              className="w-full border border-[#33404d] bg-[#090c11] px-3.5 py-3 text-white outline-none focus:border-[#4dd0b5] focus:ring-2 focus:ring-[#4dd0b5]/20 focus-visible:ring-2 focus-visible:ring-[#4dd0b5]"
            />
          </label>
          {error ? <Notice variant="error">{error}</Notice> : null}
          <button
            type="submit"
            disabled={submitting}
            className="cursor-pointer border-0 bg-[#4dd0b5] p-3.5 font-extrabold text-[#07110f] transition-opacity hover:brightness-105 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#4dd0b5] disabled:opacity-50"
          >
            {submitting ? 'Verifying...' : 'Verify'}
          </button>
        </form>
      </section>
    </main>
  )
}
