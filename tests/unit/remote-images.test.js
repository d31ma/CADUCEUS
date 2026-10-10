import { describe, expect, it } from 'bun:test'
import {
  extractRemoteImageUrls,
  fetchRemoteImage,
  isPublicAddress,
} from '@/shared/mailbox/remote-images.js'

const png = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 0])

describe('remote image proxy boundary', () => {
  it('extracts unique image attributes, srcsets, and CSS URLs without collecting links', () => {
    expect(extractRemoteImageUrls(`
      <style>.hero{background:url('//cdn.example/hero.webp')}</style>
      <img src="https://cdn.example/a.png" srcset="https://cdn.example/a.png 1x, https://cdn.example/a@2x.png 2x">
      <table background="http://images.example/tile.gif"><a href="https://evil.example/page">open</a></table>
    `)).toEqual([
      'https://cdn.example/a.png',
      'https://cdn.example/a@2x.png',
      'http://images.example/tile.gif',
      'https://cdn.example/hero.webp',
    ])
  })

  it('rejects private, reserved, documentation, mapped, and non-IP addresses', () => {
    for (const address of [
      '127.0.0.1', '10.0.0.1', '100.64.0.1', '169.254.1.1', '172.20.0.1',
      '192.168.1.1', '192.0.2.5', '198.51.100.1', '203.0.113.1', '224.0.0.1',
      '::1', '0:0:0:0:0:0:0:1', 'fc00::1', 'fe80::1', '2001:db8::1',
      '::ffff:127.0.0.1', '::ffff:7f00:1', 'not-an-ip',
    ]) expect(isPublicAddress(address)).toBe(false)
    expect(isPublicAddress('93.184.216.34')).toBe(true)
    expect(isPublicAddress('2606:2800:220:1:248:1893:25c8:1946')).toBe(true)
  })

  it('pins a public resolution and accepts only signature-matching raster bytes', async () => {
    const calls = []
    const result = await fetchRemoteImage('https://images.example/pixel.png', {
      lookup: async () => [{ address: '93.184.216.34', family: 4 }],
      request: async (url, address, family, maxBytes) => {
        calls.push({ url: url.href, address, family, maxBytes })
        return { statusCode: 200, contentType: 'image/png; charset=binary', location: '', body: png }
      },
    })
    expect(calls).toEqual([{
      url: 'https://images.example/pixel.png',
      address: '93.184.216.34',
      family: 4,
      maxBytes: 4 * 1024 * 1024,
    }])
    expect(result).toEqual({
      url: 'https://images.example/pixel.png',
      contentType: 'image/png',
      contentBase64: png.toString('base64'),
      size: png.byteLength,
    })
  })

  it('fails closed for mixed public/private DNS, redirect rebinding, credentials, ports, and false MIME types', async () => {
    const request = async () => ({ statusCode: 200, contentType: 'image/png', location: '', body: png })
    await expect(fetchRemoteImage('https://images.example/a.png', {
      lookup: async () => [
        { address: '93.184.216.34', family: 4 },
        { address: '127.0.0.1', family: 4 },
      ],
      request,
    })).rejects.toThrow(/public addresses/)

    await expect(fetchRemoteImage('https://images.example/a.png', {
      lookup: async host => host === 'images.example'
        ? [{ address: '93.184.216.34', family: 4 }]
        : [{ address: '10.0.0.1', family: 4 }],
      request: async () => ({ statusCode: 302, location: 'http://internal.example/a.png', contentType: '', body: Buffer.alloc(0) }),
    })).rejects.toThrow(/public addresses/)

    await expect(fetchRemoteImage('https://user:secret@images.example/a.png', { lookup: async () => [], request })).rejects.toThrow(/credentials/)
    await expect(fetchRemoteImage('https://images.example:8443/a.png', { lookup: async () => [], request })).rejects.toThrow(/standard web port/)
    await expect(fetchRemoteImage('https://images.example/a.png', {
      lookup: async () => [{ address: '93.184.216.34', family: 4 }],
      request: async () => ({ statusCode: 200, contentType: 'image/png', location: '', body: Buffer.from('<html>not an image</html>') }),
    })).rejects.toThrow(/supported raster image/)
  })
})
