import { lookup as dnsLookup } from 'node:dns/promises'
import http from 'node:http'
import https from 'node:https'
import { BlockList, isIP } from 'node:net'
import { DomUtils, parseDocument } from '../../deps/npm/htmlparser2.mjs'

export class RemoteImages {
  static MAX_REMOTE_IMAGE_BYTES = 4 * 1024 * 1024

  static MAX_REMOTE_IMAGES_PER_REQUEST = 6

  static #IMAGE_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/avif'])

  static #REDIRECT_CODES = new Set([301, 302, 303, 307, 308])

  static #PUBLIC_IPV6 = RemoteImages.#publicIpv6()

  /** @returns {BlockList} The global unicast IPv6 range. */
  static #publicIpv6() {
    const list = new BlockList()
    list.addSubnet('2000::', 3, 'ipv6')
    return list
  }

  /**
   * Extract only network-backed image candidates from stored message HTML.
   * The route compares caller input against this set so it cannot become a
   * general-purpose authenticated web proxy.
   *
   * @param {unknown} html
   * @returns {string[]}
   */
  static extractRemoteImageUrls(html) {
    if (typeof html !== 'string' || !html) return []
    const document = parseDocument(html, { decodeEntities: true })
    const urls = []
    const add = (value) => {
      const normalized = RemoteImages.#normalizeRemoteUrl(value)
      if (normalized && !urls.includes(normalized) && urls.length < 64) urls.push(normalized)
    }

    for (const node of DomUtils.findAll(node => Boolean(node?.attribs), document.children)) {
      const attributes = node.attribs || {}
      for (const name of ['src', 'background', 'poster']) add(attributes[name])
      for (const entry of String(attributes.srcset || '').split(',')) {
        add(entry.trim().split(/\s+/)[0])
      }
      for (const match of String(attributes.style || '').matchAll(/url\(\s*['"]?([^'"\)]+)['"]?\s*\)/gi)) {
        add(match[1])
      }
    }

    for (const style of DomUtils.findAll(node => node?.name === 'style', document.children)) {
      for (const match of DomUtils.textContent(style).matchAll(/url\(\s*['"]?([^'"\)]+)['"]?\s*\)/gi)) {
        add(match[1])
      }
    }
    return urls
  }

  /** @param {unknown} value */
  static #normalizeRemoteUrl(value) {
    const raw = typeof value === 'string' ? value.trim() : ''
    if (!raw) return ''
    try {
      const parsed = new URL(raw.startsWith('//') ? `https:${raw}` : raw)
      return ['http:', 'https:'].includes(parsed.protocol) ? parsed.href : ''
    } catch {
      return ''
    }
  }

  /**
   * Fetch one remote image without forwarding browser identity, cookies, or a
   * referrer. Every redirect is re-resolved and the selected public address is
   * pinned into the socket lookup to close DNS-rebinding races.
   *
   * @param {string} source
   * @param {{
   *   lookup?: typeof dnsLookup,
   *   request?: typeof requestPinned,
   *   maxRedirects?: number,
   *   maxBytes?: number,
   * }} [options]
   * @returns {Promise<{ url: string, contentType: string, contentBase64: string, size: number }>}
   */
  static async fetchRemoteImage(source, options = {}) {
    const lookup = options.lookup || dnsLookup
    const request = options.request || RemoteImages.#requestPinned
    const maxRedirects = Math.max(0, Math.min(5, options.maxRedirects ?? 3))
    const maxBytes = Math.max(1, Math.min(RemoteImages.MAX_REMOTE_IMAGE_BYTES, options.maxBytes ?? RemoteImages.MAX_REMOTE_IMAGE_BYTES))
    let current = RemoteImages.#parseRemoteUrl(source)

    for (let redirect = 0; redirect <= maxRedirects; redirect += 1) {
      const records = await lookup(current.hostname, { all: true, verbatim: true })
      if (!Array.isArray(records) || records.length === 0 || records.some(record => !RemoteImages.isPublicAddress(record.address))) {
        throw new Error('Remote image host did not resolve to public addresses')
      }
      const selected = records[0]
      const response = await request(current, selected.address, selected.family, maxBytes)
      if (RemoteImages.#REDIRECT_CODES.has(response.statusCode)) {
        if (!response.location || redirect === maxRedirects) throw new Error('Remote image redirect rejected')
        current = RemoteImages.#parseRemoteUrl(new URL(response.location, current).href)
        continue
      }
      if (response.statusCode < 200 || response.statusCode >= 300) throw new Error('Remote image request failed')
      const contentType = String(response.contentType || '').split(';', 1)[0].trim().toLowerCase()
      if (!RemoteImages.#IMAGE_TYPES.has(contentType) || !RemoteImages.#hasImageSignature(response.body, contentType)) {
        throw new Error('Remote resource is not a supported raster image')
      }
      return {
        url: RemoteImages.#normalizeRemoteUrl(source),
        contentType,
        contentBase64: response.body.toString('base64'),
        size: response.body.byteLength,
      }
    }
    throw new Error('Remote image redirect rejected')
  }

  /** @param {string} value */
  static #parseRemoteUrl(value) {
    const normalized = RemoteImages.#normalizeRemoteUrl(value)
    if (!normalized) throw new Error('Remote image URL must use HTTP or HTTPS')
    const parsed = new URL(normalized)
    if (parsed.username || parsed.password) throw new Error('Remote image URL must not contain credentials')
    const port = parsed.port || (parsed.protocol === 'https:' ? '443' : '80')
    if (!['80', '443'].includes(port)) throw new Error('Remote image URL must use a standard web port')
    return parsed
  }

  /**
   * Reject local, private, link-local, multicast, documentation, benchmarking,
   * and otherwise non-global address ranges for both IP families.
   *
   * @param {unknown} value
   */
  static isPublicAddress(value) {
    const address = typeof value === 'string' ? value.trim().toLowerCase() : ''
    const family = isIP(address)
    if (family === 4) {
      const octets = address.split('.').map(Number)
      const [a, b, c] = octets
      if (a === 0 || a === 10 || a === 127 || a >= 224) return false
      if (a === 100 && b >= 64 && b <= 127) return false
      if (a === 169 && b === 254) return false
      if (a === 172 && b >= 16 && b <= 31) return false
      if (a === 192 && b === 0 && c === 0) return false
      if (a === 192 && b === 0 && c === 2) return false
      if (a === 192 && b === 168) return false
      if (a === 198 && [18, 19].includes(b)) return false
      if (a === 198 && b === 51 && c === 100) return false
      if (a === 203 && b === 0 && c === 113) return false
      return octets.length === 4 && octets.every(octet => Number.isInteger(octet) && octet >= 0 && octet <= 255)
    }
    if (family !== 6 || !RemoteImages.#PUBLIC_IPV6.check(address, 'ipv6')) return false
    return !/^2001:0?db8:/i.test(address)
  }

  /**
   * @param {URL} url
   * @param {string} address
   * @param {number} family
   * @param {number} maxBytes
   */
  static #requestPinned(url, address, family, maxBytes) {
    return new Promise((resolve, reject) => {
      const transport = url.protocol === 'https:' ? https : http
      const request = transport.request({
        protocol: url.protocol,
        hostname: url.hostname,
        port: url.port || undefined,
        path: `${url.pathname}${url.search}`,
        method: 'GET',
        headers: {
          Accept: 'image/avif,image/webp,image/png,image/jpeg,image/gif',
          'Accept-Encoding': 'identity',
          'User-Agent': 'CADUCEUS-Remote-Image-Proxy/1.0',
        },
        lookup(_hostname, _options, callback) {
          callback(null, address, family)
        },
      }, response => {
        const statusCode = response.statusCode || 0
        const location = Array.isArray(response.headers.location)
          ? response.headers.location[0]
          : response.headers.location
        const contentType = Array.isArray(response.headers['content-type'])
          ? response.headers['content-type'][0]
          : response.headers['content-type']
        if (RemoteImages.#REDIRECT_CODES.has(statusCode)) {
          response.resume()
          resolve({ statusCode, location: location || '', contentType: '', body: Buffer.alloc(0) })
          return
        }
        const declared = Number(response.headers['content-length'] || 0)
        if (declared > maxBytes) {
          response.destroy()
          reject(new Error('Remote image exceeds the size limit'))
          return
        }
        const chunks = []
        let size = 0
        response.on('data', chunk => {
          size += chunk.byteLength
          if (size > maxBytes) {
            response.destroy(new Error('Remote image exceeds the size limit'))
            return
          }
          chunks.push(chunk)
        })
        response.on('end', () => resolve({ statusCode, location: '', contentType: contentType || '', body: Buffer.concat(chunks) }))
        response.on('error', reject)
      })
      request.setTimeout(10_000, () => request.destroy(new Error('Remote image request timed out')))
      request.on('error', reject)
      request.end()
    })
  }

  /** @param {Buffer} bytes @param {string} contentType */
  static #hasImageSignature(bytes, contentType) {
    if (contentType === 'image/png') return bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))
    if (contentType === 'image/jpeg') return bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[bytes.length - 2] === 0xff && bytes[bytes.length - 1] === 0xd9
    if (contentType === 'image/gif') return ['GIF87a', 'GIF89a'].includes(bytes.subarray(0, 6).toString('ascii'))
    if (contentType === 'image/webp') return bytes.subarray(0, 4).toString('ascii') === 'RIFF' && bytes.subarray(8, 12).toString('ascii') === 'WEBP'
    if (contentType === 'image/avif') return bytes.subarray(4, 8).toString('ascii') === 'ftyp' && /^(?:avif|avis)$/.test(bytes.subarray(8, 12).toString('ascii'))
    return false
  }
}
