import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, test } from 'vitest'

const projectRoot = resolve(import.meta.dirname, '..')

describe('Google OAuth branding content', () => {
  test('exposes the app name, purpose, and privacy link without JavaScript', () => {
    const homepage = readFileSync(resolve(projectRoot, 'index.html'), 'utf8')

    expect(homepage).toContain('<h1>Seven Spade</h1>')
    expect(homepage).toContain('real-time multiplayer card game')
    expect(homepage).toContain('Custom rooms support 2-8 players')
    expect(homepage).toContain('achievements, daily rewards, event progress, and cosmetic skins')
    expect(homepage).toContain('href="/privacy"')
    expect(homepage).toContain('class="app-loading-cover"')
    expect(homepage).toContain('@keyframes app-loading-spin')
  })

  test('explains how Google identity data is handled', () => {
    const privacy = readFileSync(resolve(projectRoot, 'public/legal/privacy.html'), 'utf8')
    const normalizedPrivacy = privacy.replace(/\s+/g, ' ')

    expect(privacy).toContain('Google user data')
    expect(privacy).toContain('<code>openid</code>')
    expect(privacy).toContain('<code>email</code>')
    expect(privacy).toContain('<code>profile</code>')
    expect(normalizedPrivacy).toContain('do not request access to your Google contacts')
    expect(normalizedPrivacy).toContain('Google access and ID tokens are not stored')
  })
})
