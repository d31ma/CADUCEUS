#!/usr/bin/env bun

import { createHash } from 'node:crypto'
import {
  copyFileSync,
  createReadStream,
  createWriteStream,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs'
import { dirname, resolve, sep } from 'node:path'
import { tmpdir } from 'node:os'
import { Readable } from 'node:stream'
import { pipeline } from 'node:stream/promises'
import {
  OPUS_EXTENSION_LANGUAGE_CODES,
  SMALL100_LANGUAGE_CODES,
  TRANSLATION_LANGUAGE_CODES,
} from '../client/shared/scripts/translation-languages.js'

const root = resolve(import.meta.dir, '..')
const lockPath = resolve(root, 'translation-models.lock.json')
const lock = JSON.parse(readFileSync(lockPath, 'utf8'))
const assetRoot = resolve(root, 'translation-assets')
const modelRoot = resolve(assetRoot, 'models')
const runtimeRoot = resolve(assetRoot, 'runtime')
const publicManifestPath = resolve(assetRoot, 'catalog.json')
const mode = process.argv[2] || 'vendor'

if (!['vendor', '--check-lock', '--verify'].includes(mode)) {
  console.error('Usage: bun scripts/vendor-translation-assets.mjs [--check-lock|--verify]')
  process.exit(64)
}

validateLock(lock)
const maxAssetBytes = lock.engine.maxAssetBytes
const lockedModels = [lock.model, ...lock.fallbackModels]
verifyPackageVersion('@huggingface/transformers', lock.engine.package, lock.engine.version)
verifyPackageVersion('onnxruntime-web', lock.engine.runtimePackage, lock.engine.runtimeVersion)
await verifyRuntimeSources(lock.runtime.files)

if (mode === '--check-lock') {
  console.log(`Translation lock is valid: ${lockedModels.map(model => `${model.id}@${model.revision}`).join(', ')}`)
  console.log(`Pinned payload: ${bytesLabel(payloadBytes())}`)
  process.exit(0)
}

if (mode === '--verify') {
  await verifyInstalledAssets()
  const count = await verifyPublicCatalog()
  console.log(`Verified ${count} published self-hosted translation asset files.`)
  process.exit(0)
}

const staging = mkdtempSync(resolve(tmpdir(), 'caduceus-translation-assets-'))
try {
  for (const model of lockedModels) await vendorModelFiles(staging, model)
  vendorRuntimeFiles(staging)
  await verifyInstalledAssets()
  pruneUnpinnedModelFiles()
  removeLegacyClientAssets()
  await writePublicManifest()
  console.log(`Vendored ${lockedModels.length} translation models and ONNX Runtime Web (${bytesLabel(payloadBytes())}).`)
} finally {
  rmSync(staging, { recursive: true, force: true })
}

function validateLock(value) {
  if (value?.schemaVersion !== 1) throw new Error('Unsupported translation lock schema.')
  if (value?.engine?.modelFileDtype !== 'fp32') throw new Error('Translation lock must select the model\'s standard ONNX filenames.')
  if (value?.engine?.weightQuantization !== 'int8') throw new Error('Translation lock must use the qualified int8 weights.')
  if (!Number.isSafeInteger(value?.engine?.maxAssetBytes) || value.engine.maxAssetBytes <= 0 || value.engine.maxAssetBytes >= 16 * 1024 * 1024) {
    throw new Error('Translation asset chunks must remain below Tachyon\'s 16 MiB limit.')
  }
  if (!Array.isArray(value?.fallbackModels) || value.fallbackModels.length !== 2) {
    throw new Error('Translation lock must pin both directional OPUS fallback models.')
  }
  const models = [value.model, ...value.fallbackModels]
  if (new Set(models.map(model => model?.id)).size !== models.length) throw new Error('Translation model ids must be unique.')
  for (const model of models) validateModel(model)
  if (value.model.license !== 'MIT') throw new Error('Primary translation model license changed from MIT.')
  for (const model of value.fallbackModels) {
    if (model.license !== 'Apache-2.0') throw new Error(`Fallback translation model license changed for ${model.id}.`)
  }
  const files = [...models.flatMap(model => model.files || []), ...(value?.runtime?.files || [])]
  if (!files.length) throw new Error('Translation lock contains no assets.')
  for (const file of files) {
    safeRelative(file?.path)
    if (!Number.isSafeInteger(file?.bytes) || file.bytes <= 0) throw new Error(`Invalid byte count for ${file?.path || '<unknown>'}.`)
    if (!/^[a-f0-9]{64}$/.test(file?.sha256 || '')) throw new Error(`Invalid SHA-256 for ${file?.path || '<unknown>'}.`)
  }
  validateLanguages(value.model.languages, SMALL100_LANGUAGE_CODES, 'primary model')
  for (const model of value.fallbackModels) validateLanguages(model.languages, OPUS_EXTENSION_LANGUAGE_CODES, model.id)
  const coveredLanguages = new Set(models.flatMap(model => model.languages))
  validateLanguages([...coveredLanguages], TRANSLATION_LANGUAGE_CODES, 'translation lock')
}

function validateModel(model) {
  if (!/^[A-Za-z0-9._-]+\/[A-Za-z0-9._-]+$/.test(model?.id || '')) throw new Error('Invalid translation model id.')
  if (!/^[a-f0-9]{40}$/.test(model?.revision || '')) throw new Error(`Translation model revision must be a full commit SHA for ${model?.id || '<unknown>'}.`)
  if (!['fp32', 'int8'].includes(model?.fileDtype || lock.engine.modelFileDtype)) throw new Error(`Invalid model file dtype for ${model.id}.`)
  if (!Array.isArray(model?.files) || !model.files.length) throw new Error(`Translation model ${model?.id || '<unknown>'} contains no assets.`)
}

function validateLanguages(actual, expected, label) {
  if (!Array.isArray(actual) || actual.length !== expected.length) throw new Error(`${label} language count differs from its catalog.`)
  for (const required of expected) {
    if (!actual.includes(required)) throw new Error(`${label} is missing ${required}.`)
  }
}

function verifyPackageVersion(label, packageName, expected) {
  const packagePath = resolve(root, 'node_modules', packageName, 'package.json')
  const metadata = JSON.parse(readFileSync(packagePath, 'utf8'))
  if (metadata.version !== expected) throw new Error(`${label} ${metadata.version} does not match translation lock ${expected}.`)
}

async function verifyRuntimeSources(files) {
  for (const file of files) {
    const source = resolve(root, 'node_modules/onnxruntime-web/dist', safeRelative(file.path))
    await verifyFile(source, file, 'installed ONNX runtime package')
  }
}

async function vendorModelFiles(staging, model) {
  const modelTarget = resolve(modelRoot, ...model.id.split('/'))
  for (const file of model.files) {
    const relative = safeRelative(file.path)
    const destination = resolve(modelTarget, relative)
    if (file.bytes > maxAssetBytes && await matchesChunked(destination, file)) continue
    if (file.bytes <= maxAssetBytes && await matches(destination, file)) continue
    const staged = resolve(staging, 'models', ...model.id.split('/'), relative)
    mkdirSync(dirname(staged), { recursive: true })
    if (await matches(destination, file)) {
      copyFileSync(destination, staged)
    } else {
      const encodedPath = relative.split('/').map(encodeURIComponent).join('/')
      const url = `${model.source}/resolve/${model.revision}/${encodedPath}`
      console.log(`Downloading ${file.path} (${bytesLabel(file.bytes)})`)
      const response = await fetch(url, { redirect: 'follow' })
      if (!response.ok || !response.body) throw new Error(`Could not download ${file.path}: HTTP ${response.status}.`)
      // Stream large ONNX files directly to disk. Bun.write(Response) can retain
      // the redirected Hugging Face body indefinitely and buffering a 344 MiB
      // model shard would make multi-architecture container builds fragile.
      await pipeline(Readable.fromWeb(/** @type {any} */ (response.body)), createWriteStream(staged, { mode: 0o644 }))
    }
    await verifyFile(staged, file, 'download')
    if (file.bytes > maxAssetBytes) {
      await installChunkedFile(staged, destination, file)
    } else {
      removeChunkedFile(destination)
      mkdirSync(dirname(destination), { recursive: true })
      copyFileSync(staged, destination)
    }
  }
}

function vendorRuntimeFiles(staging) {
  // This directory contains generated runtime assets only. Recreate it so an
  // older lock cannot leave an oversized or unused WASM variant publishable.
  rmSync(runtimeRoot, { recursive: true, force: true })
  mkdirSync(runtimeRoot, { recursive: true })
  for (const file of lock.runtime.files) {
    const relative = safeRelative(file.path)
    const source = resolve(root, 'node_modules/onnxruntime-web/dist', relative)
    const staged = resolve(staging, 'runtime', relative)
    const destination = resolve(runtimeRoot, relative)
    mkdirSync(dirname(staged), { recursive: true })
    copyFileSync(source, staged)
    mkdirSync(dirname(destination), { recursive: true })
    copyFileSync(staged, destination)
  }
}

async function verifyInstalledAssets() {
  for (const model of lockedModels) {
    const modelTarget = resolve(modelRoot, ...model.id.split('/'))
    for (const file of model.files) {
      const destination = resolve(modelTarget, safeRelative(file.path))
      if (file.bytes > maxAssetBytes) await verifyChunkedFile(destination, file)
      else await verifyFile(destination, file, 'self-hosted model')
    }
  }
  for (const file of lock.runtime.files) {
    await verifyFile(resolve(runtimeRoot, safeRelative(file.path)), file, 'self-hosted runtime')
  }
}

async function writePublicManifest() {
  const digest = createHash('sha256').update(readFileSync(lockPath)).digest('hex')
  const files = []
  for (const path of listFiles(assetRoot).filter(path => path !== publicManifestPath)) {
    const relative = path.slice(assetRoot.length + 1).split(sep).join('/')
    files.push({ path: relative, bytes: statSync(path).size, sha256: await sha256File(path) })
  }
  writeFileSync(publicManifestPath, JSON.stringify({
    schemaVersion: 2,
    models: lockedModels.map(model => ({
      id: model.id,
      revision: model.revision,
      fileDtype: model.fileDtype || lock.engine.modelFileDtype,
    })),
    weightQuantization: lock.engine.weightQuantization,
    languages: TRANSLATION_LANGUAGE_CODES,
    maxAssetBytes,
    bytes: payloadBytes(),
    lockSha256: digest,
    files,
  }, null, 2) + '\n', { mode: 0o644 })
}

async function verifyPublicCatalog() {
  if (!existsSync(publicManifestPath)) throw new Error('Missing self-hosted translation asset catalog.')
  const catalog = JSON.parse(readFileSync(publicManifestPath, 'utf8'))
  const digest = createHash('sha256').update(readFileSync(lockPath)).digest('hex')
  const expectedModels = lockedModels.map(model => ({
    id: model.id,
    revision: model.revision,
    fileDtype: model.fileDtype || lock.engine.modelFileDtype,
  }))
  if (catalog?.schemaVersion !== 2 || JSON.stringify(catalog?.models) !== JSON.stringify(expectedModels) || catalog?.lockSha256 !== digest || !Array.isArray(catalog?.files)) {
    throw new Error('Translation asset catalog does not match the lock.')
  }
  const expectedPaths = new Set(listFiles(assetRoot)
    .filter(path => path !== publicManifestPath)
    .map(path => path.slice(assetRoot.length + 1).split(sep).join('/')))
  if (catalog.files.length !== expectedPaths.size) throw new Error('Translation asset catalog file count mismatch.')
  for (const file of catalog.files) {
    safeRelative(file.path)
    if (!expectedPaths.delete(file.path)) throw new Error(`Unexpected or duplicate translation asset catalog entry: ${file.path}.`)
    await verifyFile(resolve(assetRoot, file.path), file, 'published translation asset')
  }
  if (expectedPaths.size) throw new Error('Translation asset catalog is incomplete.')
  return catalog.files.length
}

async function verifyFile(path, expected, label) {
  if (!existsSync(path)) throw new Error(`Missing ${label} file: ${expected.path}.`)
  const actualBytes = statSync(path).size
  if (actualBytes !== expected.bytes) throw new Error(`${label} size mismatch for ${expected.path}: ${actualBytes} != ${expected.bytes}.`)
  const actualSha256 = await sha256File(path)
  if (actualSha256 !== expected.sha256) throw new Error(`${label} SHA-256 mismatch for ${expected.path}.`)
}

async function matches(path, expected) {
  try {
    await verifyFile(path, expected, 'cached model')
    return true
  } catch {
    return false
  }
}

async function installChunkedFile(source, destination, expected) {
  removeChunkedFile(destination)
  rmSync(destination, { force: true })
  const manifestPath = `${destination}.parts.json`
  const chunkDirectory = `${destination}.parts`
  mkdirSync(chunkDirectory, { recursive: true })
  const chunks = []
  let index = 0
  for await (const buffer of createReadStream(source, { highWaterMark: maxAssetBytes })) {
    const name = `part-${String(index).padStart(5, '0')}.bin`
    const path = resolve(chunkDirectory, name)
    writeFileSync(path, buffer, { mode: 0o644 })
    chunks.push({
      path: `${expected.path.split('/').at(-1)}.parts/${name}`,
      bytes: buffer.length,
      sha256: createHash('sha256').update(buffer).digest('hex'),
    })
    index += 1
  }
  writeFileSync(manifestPath, JSON.stringify({
    schemaVersion: 1,
    source: expected,
    chunkBytes: maxAssetBytes,
    chunks,
  }, null, 2) + '\n', { mode: 0o644 })
  await verifyChunkedFile(destination, expected)
}

async function verifyChunkedFile(destination, expected) {
  const manifestPath = `${destination}.parts.json`
  if (!existsSync(manifestPath)) throw new Error(`Missing self-hosted model manifest: ${expected.path}.parts.json.`)
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
  if (manifest?.schemaVersion !== 1 || manifest?.source?.path !== expected.path || manifest?.source?.bytes !== expected.bytes || manifest?.source?.sha256 !== expected.sha256) {
    throw new Error(`Chunk manifest does not match translation lock for ${expected.path}.`)
  }
  if (manifest?.chunkBytes !== maxAssetBytes || !Array.isArray(manifest?.chunks) || !manifest.chunks.length) {
    throw new Error(`Invalid chunk manifest for ${expected.path}.`)
  }
  const hash = createHash('sha256')
  let bytes = 0
  for (const chunk of manifest.chunks) {
    const relative = safeRelative(chunk.path)
    if (chunk.bytes > maxAssetBytes) throw new Error(`Translation chunk exceeds Tachyon's asset limit: ${chunk.path}.`)
    const path = resolve(dirname(manifestPath), relative)
    await verifyFile(path, chunk, 'self-hosted model chunk')
    for await (const buffer of createReadStream(path)) hash.update(buffer)
    bytes += chunk.bytes
  }
  if (bytes !== expected.bytes || hash.digest('hex') !== expected.sha256) {
    throw new Error(`Reconstructed checksum mismatch for ${expected.path}.`)
  }
}

async function matchesChunked(destination, expected) {
  try {
    await verifyChunkedFile(destination, expected)
    return true
  } catch {
    return false
  }
}

function removeChunkedFile(destination) {
  rmSync(`${destination}.parts.json`, { force: true })
  rmSync(`${destination}.parts`, { recursive: true, force: true })
}

function removeLegacyClientAssets() {
  const legacyRoot = resolve(root, 'client/shared/assets')
  rmSync(resolve(legacyRoot, 'translation-models'), { recursive: true, force: true })
  rmSync(resolve(legacyRoot, 'translation-runtime'), { recursive: true, force: true })
  rmSync(resolve(legacyRoot, 'translation-assets.json'), { force: true })
}

function pruneUnpinnedModelFiles() {
  const expected = new Set()
  for (const model of lockedModels) {
    const modelTarget = resolve(modelRoot, ...model.id.split('/'))
    for (const file of model.files) {
      const destination = resolve(modelTarget, safeRelative(file.path))
      if (file.bytes <= maxAssetBytes) {
        expected.add(destination)
        continue
      }
      const manifestPath = `${destination}.parts.json`
      expected.add(manifestPath)
      const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
      for (const chunk of manifest.chunks) {
        expected.add(resolve(dirname(manifestPath), safeRelative(chunk.path)))
      }
    }
  }
  for (const file of listFiles(modelRoot)) {
    if (!expected.has(file)) rmSync(file, { force: true })
  }
}

function listFiles(directory) {
  if (!existsSync(directory)) return []
  const files = []
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name)
    if (entry.isDirectory()) files.push(...listFiles(path))
    else if (entry.isFile()) files.push(path)
  }
  return files.sort()
}

function safeRelative(value) {
  const normalized = String(value || '').replaceAll('\\', '/')
  if (!normalized || normalized.startsWith('/') || normalized.includes('../') || normalized.includes('/..') || normalized.includes('\0')) {
    throw new Error(`Unsafe translation asset path: ${value || '<empty>'}.`)
  }
  const resolved = resolve(root, normalized)
  if (!resolved.startsWith(root + sep)) throw new Error(`Translation asset escapes the project root: ${value}.`)
  return normalized
}

function sha256File(path) {
  return new Promise((resolveHash, reject) => {
    const hash = createHash('sha256')
    const stream = createReadStream(path)
    stream.on('data', chunk => hash.update(chunk))
    stream.once('error', reject)
    stream.once('end', () => resolveHash(hash.digest('hex')))
  })
}

function totalBytes(files) {
  return files.reduce((sum, file) => sum + file.bytes, 0)
}

function payloadBytes() {
  return totalBytes(lockedModels.flatMap(model => model.files)) + totalBytes(lock.runtime.files)
}

function bytesLabel(bytes) {
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
}
