import { describe, expect, it } from 'bun:test'
import { EmailContentProtectionUtils } from '../../client/shared/utils/EmailContentProtectionUtils.js'

const analyzeLinkDestination = (...args) => EmailContentProtectionUtils.analyzeLinkDestination(...args)
const buildProtectedEmailDocument = (...args) => EmailContentProtectionUtils.buildProtectedEmailDocument(...args)
const isKnownTrackingUrl = (...args) => EmailContentProtectionUtils.isKnownTrackingUrl(...args)
const isRemoteContentUrl = (...args) => EmailContentProtectionUtils.isRemoteContentUrl(...args)
const protectCss = (...args) => EmailContentProtectionUtils.protectCss(...args)
const safeExternalLinkTarget = (...args) => EmailContentProtectionUtils.safeExternalLinkTarget(...args)

describe('email remote-content protection', () => {
  it('distinguishes network content from inline and content-id resources', () => {
    expect(isRemoteContentUrl('https://images.example.test/hero.png')).toBe(true)
    expect(isRemoteContentUrl('//images.example.test/hero.png')).toBe(true)
    expect(isRemoteContentUrl('cid:logo@example.test')).toBe(false)
    expect(isRemoteContentUrl('data:image/png;base64,AA==')).toBe(false)
  })

  it('recognizes provider and CADUCEUS tracking endpoints', () => {
    expect(isKnownTrackingUrl('https://sdr.salesloft.com/email_trackers/id/open.gif')).toBe(true)
    expect(isKnownTrackingUrl('https://mail.example.test/track/open?id=opaque')).toBe(true)
    expect(isKnownTrackingUrl('https://images.example.test/newsletter/hero.png')).toBe(false)
  })

  it('removes CSS network vectors before consent', () => {
    const result = protectCss(`
      @import url('https://styles.example.test/mail.css');
      @font-face { font-family: leak; src: url(https://fonts.example.test/font.woff2); }
      .hero { background-image: url("https://images.example.test/hero.png"); }
      .logo { background-image: url(data:image/png;base64,AA==); }
    `)

    expect(result.css).not.toContain('styles.example.test')
    expect(result.css).not.toContain('fonts.example.test')
    expect(result.css).not.toContain('images.example.test')
    expect(result.css).toContain('data:image/png')
    expect(result.remoteCount).toBe(1)
    expect(result.blockedCount).toBe(1)
  })

  it('uses inert proxy data after consent and still removes trackers', () => {
    const result = protectCss(
      '.hero{background:url(https://images.example.test/hero.png)}.pixel{background:url(https://mailtrack.io/trace/id)}',
      {
        allowRemote: true,
        remoteSources: { 'https://images.example.test/hero.png': 'data:image/png;base64,AA==' },
      },
    )

    expect(result.css).toContain('data:image/png;base64,AA==')
    expect(result.css).not.toContain('images.example.test/hero.png')
    expect(result.css).not.toContain('mailtrack.io')
    expect(result.remoteCount).toBe(2)
    expect(result.blockedCount).toBe(0)
    expect(result.trackingCount).toBe(1)
    expect(result.remoteUrls).toEqual(['https://images.example.test/hero.png'])
  })

  it('keeps consented images blocked until proxy data is available', () => {
    const result = protectCss('.hero{background:url(https://images.example.test/hero.png)}', { allowRemote: true })
    expect(result.css).not.toContain('images.example.test')
    expect(result.blockedCount).toBe(1)
  })

  it('uses a fixed light or dark sandbox palette without weakening CSP', () => {
    const dark = buildProtectedEmailDocument('<p>Hello</p>', { colorMode: 'dark' }).documentHtml
    expect(dark).toContain('color-scheme:dark')
    expect(dark).toContain('background:#181a1b')
    expect(dark).toContain("default-src 'none'")
    const light = buildProtectedEmailDocument('<p>Hello</p>', { colorMode: 'light' }).documentHtml
    expect(light).toContain('color-scheme:light')
    expect(light).toContain('background:#fff')
  })

  it('resolves authenticated CID raster sources and removes unresolved or active ones', () => {
    const result = protectCss(
      '.logo{background:url(cid:logo@example.test)}.missing{background:url(cid:missing@example.test)}',
      { cidSources: { 'logo@example.test': 'data:image/png;base64,AA==' } },
    )
    expect(result.css).toContain('data:image/png;base64,AA==')
    expect(result.css).not.toContain('cid:')

    const active = protectCss('.bad{background:url(cid:bad@example.test)}', {
      cidSources: { 'bad@example.test': 'data:image/svg+xml;base64,AA==' },
    })
    expect(active.css).not.toContain('image/svg+xml')
  })

  it('warns for disguised and internationalized destinations but not matching ordinary links', () => {
    expect(analyzeLinkDestination('https://accounts.example.test/login', 'accounts.example.test').suspicious).toBe(false)

    const mismatch = analyzeLinkDestination('https://evil.example.test/login', 'https://accounts.example.test/login')
    expect(mismatch.suspicious).toBe(true)
    expect(mismatch.reason).toContain('does not match')
    expect(mismatch.destinationHost).toBe('evil.example.test')
    expect(mismatch.displayedHost).toBe('accounts.example.test')

    const homograph = analyzeLinkDestination('https://раypal.example/path', 'Open account')
    expect(homograph.suspicious).toBe(true)
    expect(homograph.destinationHost).toContain('xn--')
    expect(homograph.reason).toContain('internationalized')
    expect(safeExternalLinkTarget('https://user:secret@example.test')).toBe('')
    expect(safeExternalLinkTarget('javascript:alert(1)')).toBe('')
  })
})
