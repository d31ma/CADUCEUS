#!/usr/bin/env bun

import {
  appendFileSync,
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  realpathSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs'
import { dirname, relative, resolve } from 'node:path'
import { tmpdir } from 'node:os'
import {
  AutoModelForSeq2SeqLM,
  AutoTokenizer,
  Tensor,
  env,
} from '@huggingface/transformers'
import { parseDocument } from 'htmlparser2'

const root = resolve(import.meta.dir, '..')
const modelId = 'casawolice/small100-onnx'
const assetModelRoot = resolve(root, 'translation-assets/models', modelId)
const targetTokens = Object.freeze({
  ar: 'ar', cs: 'cs', de: 'de', es: 'es', 'es-419': 'es', fr: 'fr',
  gl: 'gl', he: 'he', hu: 'hu', ja: 'ja', ko: 'ko', pl: 'pl', pt: 'pt',
  'pt-BR': 'pt', ru: 'ru', tr: 'tr', uk: 'uk', 'zh-CN': 'zh', 'zh-TW': 'zh',
})

if (!existsSync(assetModelRoot)) {
  throw new Error('Run `bun run translation:vendor` before generating interface locales.')
}

const mode = process.argv[2] || '--probe'
if (!['--probe', '--inventory', '--generate'].includes(mode)) {
  throw new Error('Usage: bun scripts/generate-interface-locales.mjs [--probe|--inventory|--generate]')
}

if (mode === '--inventory') {
  const inventory = collectInterfaceLiterals()
  console.log(JSON.stringify({ count: inventory.length, values: inventory }, null, 2))
  process.exit(0)
}

const staging = mkdtempSync(resolve(tmpdir(), 'caduceus-interface-model-'))
try {
  await reconstructModel(staging)
  const translator = await loadModel(staging)
  if (mode === '--generate') {
    const inventory = collectInterfaceLiterals()
    const requested = String(process.env.CADUCEUS_LOCALES || '').split(',').map(value => value.trim()).filter(Boolean)
    const locales = requested.length ? requested : Object.keys(targetTokens)
    const catalogs = {}
    for (const locale of locales) {
      if (!targetTokens[locale]) throw new Error(`Unsupported generated interface locale: ${locale}.`)
      catalogs[locale] = await translateCatalog(translator, inventory, locale)
      console.error(`Generated ${locale}: ${inventory.length} interface phrases.`)
    }
    const requestedOutput = resolve(process.argv[3] || resolve(tmpdir(), 'caduceus-interface-catalogs.json'))
    const outputName = requestedOutput.split('/').at(-1)
    if (!outputName) throw new Error('Generated catalog output requires a filename.')
    const output = resolve(realpathSync(dirname(requestedOutput)), outputName)
    const allowedRoots = [realpathSync(tmpdir()), realpathSync('/tmp')]
    if (!allowedRoots.some(directory => output.startsWith(directory + '/'))) {
      throw new Error('Generated catalog output must remain in a temporary directory.')
    }
    writeFileSync(output, JSON.stringify({ schemaVersion: 1, inventory, catalogs }, null, 2) + '\n')
    console.log(output)
  } else {
    const values = [
      'Compose a new message', 'Settings', 'Search all mail', 'Actions',
      'Account settings', 'Save', 'Cancel', 'Appearance',
    ]
    const translated = []
    for (const value of values) translated.push(...await translateBatch(translator, [value], 'fr'))
    console.log(JSON.stringify({ values, translated }, null, 2))
  }
} finally {
  rmSync(staging, { recursive: true, force: true })
}

async function translateCatalog(translator, inventory, locale) {
  const catalog = {}
  const batchSize = 64
  for (let start = 0; start < inventory.length; start += batchSize) {
    const source = inventory.slice(start, start + batchSize)
    const translated = await translateBatch(translator, source, locale)
    if (translated.length !== source.length) throw new Error(`Translation batch size mismatch for ${locale}.`)
    for (let index = 0; index < source.length; index += 1) {
      const value = String(translated[index] || '').replace(/\s+/g, ' ').trim()
      catalog[source[index]] = value || source[index]
    }
  }
  return catalog
}

function collectInterfaceLiterals() {
  const values = new Set()
  for (const path of listFiles(resolve(root, 'client')).filter(path => path.endsWith('.html'))) {
    const document = parseDocument(readFileSync(path, 'utf8'), { decodeEntities: true })
    collectNodeLiterals(document, values, false)
  }
  return [...values].sort((left, right) => left.localeCompare(right))
}

function collectNodeLiterals(node, values, excluded) {
  const name = String(node?.name || '').toLowerCase()
  const skip = excluded || ['script', 'style', 'code', 'pre'].includes(name)
  if (!skip && node?.type === 'text') addLiteral(values, node.data)
  if (!skip && node?.attribs) {
    for (const name of ['aria-label', 'title', 'placeholder', 'alt']) {
      if (Object.hasOwn(node.attribs, name)) addLiteral(values, node.attribs[name])
    }
  }
  for (const child of node?.children || []) collectNodeLiterals(child, values, skip)
}

function addLiteral(values, candidate) {
  const value = String(candidate || '').replace(/\s+/g, ' ').trim()
  if (
    !value || !/[A-Za-z]/.test(value) || /^[A-Z0-9._@:/+-]+$/.test(value) ||
    /[{}<>$`]|\s\+\s|=>|\|\||&&/.test(value) ||
    /^[,.;:]|^(?:class|rows)=/i.test(value)
  ) return
  values.add(value)
}

async function reconstructModel(staging) {
  const target = resolve(staging, modelId)
  for (const path of listFiles(assetModelRoot)) {
    const sourceRelative = relative(assetModelRoot, path)
    if (sourceRelative.endsWith('.parts.json') || sourceRelative.includes('.onnx.parts/')) continue
    const destination = resolve(target, sourceRelative)
    mkdirSync(dirname(destination), { recursive: true })
    copyFileSync(path, destination)
  }
  for (const manifestPath of listFiles(assetModelRoot).filter(path => path.endsWith('.onnx.parts.json'))) {
    const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
    const sourceRelative = relative(assetModelRoot, manifestPath).replace(/\.parts\.json$/, '')
    const destination = resolve(target, sourceRelative)
    mkdirSync(dirname(destination), { recursive: true })
    writeFileSync(destination, new Uint8Array(), { mode: 0o600 })
    for (const chunk of manifest.chunks) {
      appendFileSync(destination, readFileSync(resolve(dirname(manifestPath), chunk.path)))
    }
  }
}

async function loadModel(staging) {
  env.allowLocalModels = true
  env.allowRemoteModels = false
  env.localModelPath = `${staging}/`
  const options = { local_files_only: true }
  const [tokenizer, model] = await Promise.all([
    AutoTokenizer.from_pretrained(modelId, options),
    AutoModelForSeq2SeqLM.from_pretrained(modelId, {
      ...options,
      device: 'cpu',
      dtype: 'fp32',
    }),
  ])
  const languageTokens = JSON.parse(readFileSync(resolve(staging, modelId, 'lang_tokens.json'), 'utf8')).lang_to_id
  return { tokenizer, model, languageTokens }
}

async function translateBatch(translator, values, locale) {
  const language = targetTokens[locale]
  if (!language) throw new Error(`Unsupported generated interface locale: ${locale}.`)
  const targetId = translator.languageTokens[language]
  if (!Number.isSafeInteger(targetId)) throw new Error(`Missing SMaLL-100 target token for ${locale}.`)
  // Bare UI labels can be decoded as an open-ended synonym list. A terminal
  // full stop gives the translation model an explicit sentence boundary; the
  // publisher removes this synthetic punctuation from the catalog value.
  const modelInputs = values.map(value => /[.!?…。！？]$/u.test(value.trim()) ? value : `${value}.`)
  const encoded = await translator.tokenizer(modelInputs, { padding: true, truncation: true, max_length: 128 })
  const [batch, width] = encoded.input_ids.dims
  const ids = new BigInt64Array(batch * (width + 1))
  const mask = new BigInt64Array(ids.length)
  for (let row = 0; row < batch; row += 1) {
    const destination = row * (width + 1)
    ids[destination] = BigInt(targetId)
    mask[destination] = 1n
    ids.set(encoded.input_ids.data.subarray(row * width, (row + 1) * width), destination + 1)
    mask.set(encoded.attention_mask.data.subarray(row * width, (row + 1) * width), destination + 1)
  }
  const generated = await translator.model.generate({
    input_ids: new Tensor('int64', ids, [batch, width + 1]),
    attention_mask: new Tensor('int64', mask, [batch, width + 1]),
    max_new_tokens: 64,
    // transformers.js 4.2 currently collapses/reorders hypotheses when beam
    // search and batched M2M generation are combined. Greedy decoding retains
    // one output per input row; the publisher rejects low-quality results.
    num_beams: 1,
    no_repeat_ngram_size: 3,
    repetition_penalty: 1.2,
  })
  return translator.tokenizer.batch_decode(generated, { skip_special_tokens: true })
}

function listFiles(directory) {
  if (!existsSync(directory)) return []
  const files = []
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name)
    if (entry.isDirectory()) files.push(...listFiles(path))
    else if (entry.isFile() && statSync(path).isFile()) files.push(path)
  }
  return files.sort()
}
