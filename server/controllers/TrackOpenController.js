import { getApp } from '../../../app.js'
import { readTrackingToken } from '../../../features/delivery/services/tracking-tokens.js'

/** 1×1 transparent GIF (43 bytes) served as the tracking pixel. */
const PIXEL = Buffer.from([
  0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
  0x01, 0x00, 0x80, 0x00, 0x00, 0xFF, 0xFF, 0xFF,
  0x00, 0x00, 0x00, 0x21, 0xF9, 0x04, 0x00, 0x00,
  0x00, 0x00, 0x00, 0x2C, 0x00, 0x00, 0x00, 0x00,
  0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
  0x01, 0x00, 0x3B,
])

const PIXEL_HEADERS = {
  'Content-Type': 'image/gif',
  'Cache-Control': 'no-cache, no-store, must-revalidate',
  'Content-Security-Policy': "default-src 'none'",
  'Cross-Origin-Resource-Policy': 'cross-origin',
  'Referrer-Policy': 'no-referrer',
  'X-Content-Type-Options': 'nosniff',
  Pragma: 'no-cache',
  Expires: '0',
}

@RequestRoute("/track/open")
@Public
export class TrackOpenController {
  /**
   * GET /track/open
   *
   * A valid encrypted capability records one recipient-level event. Every
   * request receives the same pixel so token validity and storage health are
   * never disclosed to an email client.
   *
   * @param {{ query?: { t?: string } }} params
   * @returns {Promise<Response>}
   */
  @GetRoute
  async get({ query }) {
    try {
      const capability = readTrackingToken(query?.t, { expectedType: 'open' })
      if (capability) {
        const app = await getApp()
        await app.tracking.recordOpen(capability)
      }
    } catch (err) {
      console.error('[track/open] Failed to process tracking capability:', err instanceof Error ? err.message : 'unknown error')
    }
    return new Response(PIXEL, { headers: PIXEL_HEADERS })
  }
}
