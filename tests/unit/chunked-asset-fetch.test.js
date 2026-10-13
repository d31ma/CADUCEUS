import { describe, expect, test } from 'bun:test'
import { createHash } from 'node:crypto'
import { ChunkedAssetFetchUtils } from '../../client/shared/utils/ChunkedAssetFetchUtils.js'

const createChunkedTranslationAssetFetch = (...args) => ChunkedAssetFetchUtils.createChunkedTranslationAssetFetch(...args)

const prefix = 'http://localhost/shared/assets/translation-models/casawolice/small100-onnx/onnx/test.onnx'
const digest = value => createHash('sha256').update(value).digest('hex')

function fixtureAssetFetch(chunks, mutateManifest = value => value) {
  const manifest = mutateManifest({
    schemaVersion: 1,
    source: {
      path: 'onnx/test.onnx',
      bytes: chunks.reduce((sum, value) => sum + value.byteLength, 0),
      sha256: digest(Buffer.concat(chunks)),
    },
    chunkBytes: 16,
    chunks: chunks.map((value, index) => ({
      path: `test.onnx.parts/part-${index}.bin`,
      bytes: value.byteLength,
      sha256: digest(value),
    })),
  })
  return async path => {
    if (path === 'models/casawolice/small100-onnx/onnx/test.onnx.parts.json') return Response.json(manifest)
    const index = Number(path.match(/part-(\d+)\.bin$/)?.[1])
    if (Number.isInteger(index) && chunks[index]) return new Response(chunks[index])
    return new Response('missing', { status: 404 })
  }
}

describe('chunked translation asset fetch', () => {
  test('reconstructs a normal ONNX response from verified same-origin chunks', async () => {
    const chunks = [Buffer.from('first-'), Buffer.from('second')]
    const fetchAsset = createChunkedTranslationAssetFetch(fetch, fixtureAssetFetch(chunks))
    const response = await fetchAsset(prefix)
    expect(response.status).toBe(200)
    expect(response.headers.get('content-length')).toBe('12')
    expect(await response.text()).toBe('first-second')
  })

  test('fails closed when a published chunk does not match its digest', async () => {
    const chunks = [Buffer.from('trusted')]
    const assetFetch = fixtureAssetFetch(chunks, manifest => {
      manifest.chunks[0].sha256 = '0'.repeat(64)
      return manifest
    })
    const fetchAsset = createChunkedTranslationAssetFetch(fetch, assetFetch)
    const response = await fetchAsset(prefix)
    await expect(response.arrayBuffer()).rejects.toThrow('integrity')
  })

  test('passes non-model requests through unchanged', async () => {
    const baseFetch = async input => new Response(`direct:${String(input)}`)
    const fetchAsset = createChunkedTranslationAssetFetch(baseFetch)
    expect(await (await fetchAsset('http://localhost/config.json')).text()).toBe('direct:http://localhost/config.json')
  })

  test('loads small model metadata through the authenticated asset adapter', async () => {
    const assetFetch = async path => Response.json({ path })
    const fetchAsset = createChunkedTranslationAssetFetch(fetch, assetFetch)
    const response = await fetchAsset('http://localhost/shared/assets/translation-models/casawolice/small100-onnx/config.json')
    expect(await response.json()).toEqual({ path: 'models/casawolice/small100-onnx/config.json' })
  })

  test('loads both locked OPUS model families and rejects unpinned models', async () => {
    const assetFetch = async path => Response.json({ path })
    const baseFetch = async input => new Response(`direct:${String(input)}`)
    const fetchAsset = createChunkedTranslationAssetFetch(baseFetch, assetFetch)
    const prefix = 'http://localhost/shared/assets/translation-models/'

    expect(await (await fetchAsset(`${prefix}Xenova/opus-mt-en-mul/config.json`)).json())
      .toEqual({ path: 'models/Xenova/opus-mt-en-mul/config.json' })
    expect(await (await fetchAsset(`${prefix}Xenova/opus-mt-mul-en/config.json`)).json())
      .toEqual({ path: 'models/Xenova/opus-mt-mul-en/config.json' })
    expect((await fetchAsset(`${prefix}untrusted/model/config.json`)).status).toBe(400)
  })
})
