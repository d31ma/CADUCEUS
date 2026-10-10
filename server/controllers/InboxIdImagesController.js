import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { EmailsRecords } from '../shared/mailbox/EmailsRecords.js'
import { RemoteImages } from '../shared/mailbox/RemoteImages.js'

@RequestRoute("/inbox/{id}/images")
@SignedIn
export class InboxIdImagesController {
  /** @param {string} value */
  static normalizeRequestedUrl(value) {
    try {
      const parsed = new URL(value.startsWith('//') ? `https:${value}` : value)
      return ['http:', 'https:'].includes(parsed.protocol) ? parsed.href : ''
    } catch {
      return ''
    }
  }

  static _fetchImage = RemoteImages.fetchRemoteImage

  /** Fetch bounded remote images found in one exact-owner stored message. */
  @PostRoute
  async post(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, body, auth }) => {
      const id = String(paths?.id || '')
      if (!id) return Respond.r404('Email not found')
      if (!Array.isArray(body?.urls) || body.urls.length < 1 || body.urls.length > RemoteImages.MAX_REMOTE_IMAGES_PER_REQUEST
        || body.urls.some(url => typeof url !== 'string')) {
        return Respond.r400(`urls must contain between 1 and ${RemoteImages.MAX_REMOTE_IMAGES_PER_REQUEST} remote image URLs`)
      }

      const app = await getApp()
      const [, email] = await app.emails.findById(id)
      if (!email) return Respond.r404('Email not found')
      if (!auth.domains.includes(email.domain)) return Respond.r403('Access denied')
      if (!EmailsRecords.canAccessEmail(email, auth.email)) return Respond.r404('Email not found')

      const limit = await app.rateLimit.check(['remote-images', auth.email], 60, 60_000)
      if (!limit.allowed) return Respond.r429('Too many remote image requests', limit.retryAfterSeconds)

      const allowed = new Set(RemoteImages.extractRemoteImageUrls(email.htmlBody))
      const requested = [...new Set(body.urls.map(url => InboxIdImagesController.normalizeRequestedUrl(url)))]
      if (requested.some(url => !url || !allowed.has(url))) {
        return Respond.r400('Every URL must be an image resource from this message')
      }

      const settled = await Promise.allSettled(requested.map(url => InboxIdImagesController._fetchImage(url)))
      const images = settled.flatMap(result => result.status === 'fulfilled' ? [result.value] : [])
      return { images, failed: settled.length - images.length }
    })
  }
}
