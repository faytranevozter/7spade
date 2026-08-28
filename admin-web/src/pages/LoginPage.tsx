import { useState, type FormEvent } from 'react'
import { AdminBrand } from '../components/AdminBrand'
import { Notice } from '../components/Feedback'
import { useAuth } from '../hooks/useAuth'

export function LoginPage() {
  const { signIn, error } = useAuth()
  const [submitting, setSubmitting] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    const data = new FormData(event.currentTarget)
    await signIn(String(data.get('email')), String(data.get('password')))
    setSubmitting(false)
  }

  return (
    <main className="min-h-screen grid place-items-center p-6 bg-[radial-gradient(circle_at_80%_-20%,#163538_0,transparent_34%),linear-gradient(#ffffff08_1px,transparent_1px),linear-gradient(90deg,#ffffff08_1px,transparent_1px)] bg-[size:auto,48px_48px,48px_48px] bg-[#0a0d12]">
      <section className="w-full max-w-[460px] border border-[#2a3844] border-t-[3px] border-t-[#4dd0b5] bg-[#10151cdd] p-10 shadow-[0_30px_90px_#000000aa]">
        <AdminBrand subtitle="Restricted operations console" />
        <p className="text-[#4dd0b5] font-mono font-bold text-[11px] tracking-[0.15em] mt-6 mb-2">
          AUTHORIZED PERSONNEL ONLY
        </p>
        <h1 className="my-2.5 text-3xl sm:text-4xl md:text-5xl font-bold leading-none tracking-[-0.045em] text-white">
          Admin sign in
        </h1>
        <p className="text-[#99a5b3] leading-[1.6]">
          Use your dedicated administrator identity. Player credentials are not accepted.
        </p>
        <form onSubmit={submit} className="grid gap-4.5 mt-7.5">
          <label className="grid gap-2 text-[#aeb8c4] text-sm">
            Email
            <input
              name="email"
              type="email"
              autoComplete="username"
              required
              className="w-full border border-[#33404d] bg-[#090c11] text-white px-3.5 py-3 outline-none focus:border-[#4dd0b5] focus:ring-2 focus:ring-[#4dd0b5]/20 focus-visible:ring-2 focus-visible:ring-[#4dd0b5]"
            />
          </label>
          <label className="grid gap-2 text-[#aeb8c4] text-sm">
            Password
            <input
              name="password"
              type="password"
              autoComplete="current-password"
              required
              className="w-full border border-[#33404d] bg-[#090c11] text-white px-3.5 py-3 outline-none focus:border-[#4dd0b5] focus:ring-2 focus:ring-[#4dd0b5]/20 focus-visible:ring-2 focus-visible:ring-[#4dd0b5]"
            />
          </label>
          {error ? <Notice variant="error">{error}</Notice> : null}
          <button
            type="submit"
            disabled={submitting}
            className="border-0 bg-[#4dd0b5] text-[#07110f] font-extrabold p-3.5 cursor-pointer disabled:opacity-50 transition-opacity hover:brightness-105 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#4dd0b5]"
          >
            {submitting ? 'Signing in...' : 'Sign in'}
          </button>
        </form>
      </section>
    </main>
  )
}
