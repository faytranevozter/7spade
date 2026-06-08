import { postForgotPassword, postResetPassword, postVerifyEmail, postResendVerification } from '../src/api/auth'

// Exercises the #42 recovery API client functions: correct method/path/body and
// that a non-ok response rejects. fetch is stubbed per-test.

const okJson = () =>
  Promise.resolve({ ok: true, json: () => Promise.resolve({}) } as Response)

afterEach(() => {
  jest.restoreAllMocks()
})

test('postForgotPassword posts the email', async () => {
  const fetchSpy = jest.spyOn(global, 'fetch').mockImplementation(okJson)
  await postForgotPassword('a@b.com')
  const [url, init] = fetchSpy.mock.calls[0]
  expect(String(url)).toContain('/auth/forgot-password')
  expect(init?.method).toBe('POST')
  expect(JSON.parse(String(init?.body))).toEqual({ email: 'a@b.com' })
})

test('postResetPassword posts token + password', async () => {
  const fetchSpy = jest.spyOn(global, 'fetch').mockImplementation(okJson)
  await postResetPassword('tok', 'longenough1')
  const [url, init] = fetchSpy.mock.calls[0]
  expect(String(url)).toContain('/auth/reset-password')
  expect(JSON.parse(String(init?.body))).toEqual({ token: 'tok', password: 'longenough1' })
})

test('postVerifyEmail posts the token', async () => {
  const fetchSpy = jest.spyOn(global, 'fetch').mockImplementation(okJson)
  await postVerifyEmail('vtok')
  const [url, init] = fetchSpy.mock.calls[0]
  expect(String(url)).toContain('/auth/verify-email')
  expect(JSON.parse(String(init?.body))).toEqual({ token: 'vtok' })
})

test('postResendVerification sends the bearer token', async () => {
  const fetchSpy = jest.spyOn(global, 'fetch').mockImplementation(okJson)
  await postResendVerification('jwt-abc')
  const [url, init] = fetchSpy.mock.calls[0]
  expect(String(url)).toContain('/auth/resend-verification')
  expect(init?.method).toBe('POST')
  expect((init?.headers as Record<string, string>).Authorization).toBe('Bearer jwt-abc')
})

test('a failed request rejects with an AuthApiError message', async () => {
  jest.spyOn(global, 'fetch').mockImplementation(() =>
    Promise.resolve({ ok: false, status: 400, json: () => Promise.resolve({ error: 'bad token' }) } as Response),
  )
  await expect(postResetPassword('x', 'longenough1')).rejects.toThrow('bad token')
})
