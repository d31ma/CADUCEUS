import { RouteRequest } from '../shared/http/RouteRequest.js'
import { getApp } from '../app.js'
import { readTrackingToken } from '../shared/delivery/tracking-tokens.js'

@RequestRoute("/track/click")
@Public
export class TrackClickController {
  static invalidLink() {
    return new Response('This tracked link is invalid or has expired.', {
      status: 404,
      headers: {
        'Content-Type': 'text/plain; charset=utf-8',
        'Cache-Control': 'no-store',
        'Content-Security-Policy': "default-src 'none'",
        'Referrer-Policy': 'no-referrer',
        'X-Content-Type-Options': 'nosniff',
      },
    })
  }

  /**
   * GET /track/click
   *
   * The target exists only inside an authenticated encrypted capability. A
   * caller cannot supply an arbitrary redirect destination.
   *
   * @param {{ query?: { t?: string } }} params
   * @returns {Promise<Response>}
   */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, {  }, async ({ query }) => {
      try {
        const capability = readTrackingToken(query?.t, { expectedType: 'click' })
        if (!capability || capability.type !== 'click') return TrackClickController.invalidLink()
        const app = await getApp()
        if (!await app.tracking.recordClick(capability)) return TrackClickController.invalidLink()
        return new Response(null, {
          status: 302,
          headers: {
            Location: capability.url,
            'Cache-Control': 'no-cache, no-store, must-revalidate',
            'Referrer-Policy': 'no-referrer',
            'X-Content-Type-Options': 'nosniff',
          },
        })
      } catch (err) {
        console.error('[track/click] Failed to process tracking capability:', err instanceof Error ? err.message : 'unknown error')
        return TrackClickController.invalidLink()
      }
    })
  }
}
