import { describe, expect, it } from 'bun:test'
import { composeLaunchPrefill, isManifestComposeLaunch, mailtoPrefill } from '../../client/shared/scripts/pwa-launch.js'

describe('installed-app compose launch handling', () => {
  it('parses a standards-encoded mailto URL into bounded composer fields', () => {
    expect(mailtoPrefill('mailto:alice%40example.com?cc=bob%40example.com&subject=Quarterly%20review&body=Please%20review.')).toEqual({
      to: 'alice@example.com',
      cc: 'bob@example.com',
      subject: 'Quarterly review',
      body: 'Please review.',
    })
  })

  it('rejects non-mail protocols and ignores arbitrary compose query keys', () => {
    expect(mailtoPrefill('https://attacker.test/')).toEqual({})
    expect(composeLaunchPrefill('?to=victim@example.com&subject=Injected')).toEqual({})
    expect(isManifestComposeLaunch('?to=victim@example.com')).toBe(false)
  })

  it('builds a share-target draft and accepts only ordinary web URLs', () => {
    expect(composeLaunchPrefill('?shareTitle=Useful&shareText=Read%20this&shareUrl=https%3A%2F%2Fexample.com%2Fdoc')).toEqual({
      subject: 'Useful',
      body: 'Read this\n\nhttps://example.com/doc',
    })
    expect(composeLaunchPrefill('?shareText=Nope&shareUrl=javascript%3Aalert(1)')).toEqual({ body: 'Nope' })
  })
})
