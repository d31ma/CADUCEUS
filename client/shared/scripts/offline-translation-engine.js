// @ts-check

import {
  createChunkedTranslationAssetFetch,
  fetchSelfHostedTranslationAsset,
} from './chunked-asset-fetch.js'
import {
  OPUS_EXTENSION_LANGUAGE_CODES,
  SMALL100_LANGUAGE_CODES,
  TRANSLATION_LANGUAGES,
} from './translation-languages.js'

export const OFFLINE_TRANSLATION_MODEL = 'casawolice/small100-onnx'
export const OFFLINE_TRANSLATION_MODEL_REVISION = '5c2c73ac70bee9c58f5a7ac5e84a36bee25db8ee'
export const OPUS_ENGLISH_TO_MULTILINGUAL_MODEL = 'Xenova/opus-mt-en-mul'
export const OPUS_ENGLISH_TO_MULTILINGUAL_REVISION = 'e57f9759f8e1e9ed97067bfc11e53e1eab63e57e'
export const OPUS_MULTILINGUAL_TO_ENGLISH_MODEL = 'Xenova/opus-mt-mul-en'
export const OPUS_MULTILINGUAL_TO_ENGLISH_REVISION = '72a05e47cee89c718a9db4dc70d02fef3bc39de8'
export const OFFLINE_TRANSLATION_CHUNK_CHARACTERS = 800

const small100Languages = new Set(SMALL100_LANGUAGE_CODES)
const opusExtensionLanguages = new Set(OPUS_EXTENSION_LANGUAGE_CODES)

const opusTargetTokens = Object.freeze({
  mt: 'mlt',
  mi: 'mri',
  mhr: 'mhr',
  eu: 'eus',
  pap: 'pap',
  // The compact OPUS checkpoints do not distinguish the two Mari dialects.
  // Keep Mailspring's Hill Mari identifier while using the closest local
  // model constituent instead of sending private mail to a hosted service.
  mrj: 'mhr',
  tg: 'tgk_Cyrl',
  tt: 'tat',
  te: 'tel',
  udm: 'udm',
  ky: 'kir_Cyrl',
  la: 'lat_Latn',
  eo: 'epo',
})

/** @type {Promise<any> | null} */
let vendorPromise = null
/** @type {Map<string, Promise<any>>} */
const modelPromises = new Map()

export function offlineTranslationSupported() {
  return typeof WebAssembly === 'object' && typeof fetch === 'function'
}

/**
 * Create a pair-bound facade over one process-wide multilingual pipeline.
 * Models and WASM are loaded only from the current CADUCEUS origin.
 *
 * @param {string} sourceLanguage
 * @param {string} targetLanguage
 * @param {((loaded: number) => void) | undefined} onProgress
 */
export async function createOfflineTranslator(sourceLanguage, targetLanguage, onProgress) {
  if (!offlineTranslationSupported()) {
    throw new Error('On-device translation requires WebAssembly support.')
  }

  const injected = /** @type {any} */ (globalThis).__CADUCEUS_OFFLINE_TRANSLATION_FACTORY
  if (typeof injected === 'function') {
    return injected({ sourceLanguage, targetLanguage, onProgress })
  }

  const plan = buildOfflineTranslationPlan(sourceLanguage, targetLanguage)
  const progress = createProgressReporter(plan.length, onProgress)
  const translators = await Promise.all(plan.map(async (leg, index) => {
    const translator = await loadModel(leg.model, value => progress.update(index, value))
    progress.update(index, 1)
    return translator
  }))
  return {
    async translate(value) {
      let translated = String(value || '')
      for (let index = 0; index < plan.length; index += 1) {
        translated = await translateLeg(translators[index], translated, plan[index])
      }
      return translated
    },
    // Model payloads are intentionally shared. Destroying them after each HTML
    // text node or message would force another load and exhaust browser memory.
    destroy() {},
  }
}

/**
 * Resolve the minimal self-hosted model chain for a language pair.
 *
 * @param {string} sourceLanguage
 * @param {string} targetLanguage
 * @returns {Array<{ model: 'small100' | 'opus-mul-en' | 'opus-en-mul', sourceLanguage: string, targetLanguage: string }>}
 */
export function buildOfflineTranslationPlan(sourceLanguage, targetLanguage) {
  assertSupportedLanguage(sourceLanguage)
  assertSupportedLanguage(targetLanguage)
  if (sourceLanguage === targetLanguage) return []
  if (small100Languages.has(sourceLanguage) && small100Languages.has(targetLanguage)) {
    return [{ model: 'small100', sourceLanguage, targetLanguage }]
  }

  const plan = []
  if (sourceLanguage !== 'en') {
    plan.push({
      model: opusExtensionLanguages.has(sourceLanguage) ? 'opus-mul-en' : 'small100',
      sourceLanguage,
      targetLanguage: 'en',
    })
  }
  if (targetLanguage !== 'en') {
    plan.push({
      model: opusExtensionLanguages.has(targetLanguage) ? 'opus-en-mul' : 'small100',
      sourceLanguage: 'en',
      targetLanguage,
    })
  }
  return plan
}

/**
 * @param {any} translator
 * @param {string} value
 * @param {{ model: string, sourceLanguage: string, targetLanguage: string }} leg
 */
function translateLeg(translator, value, leg) {
  return leg.model === 'small100'
    ? translateSmall100Chunks(translator, value, leg.targetLanguage)
    : translateOpusChunks(translator, value, leg)
}

/**
 * Translate bounded chunks so long emails do not exceed SMaLL-100's tokenizer
 * window. Whitespace-only separators are preserved verbatim.
 *
 * @param {{ tokenizer: any, model: any, languageTokens: Record<string, number>, Tensor: any }} translator
 * @param {string} value
 * @param {string} targetLanguage
 */
async function translateSmall100Chunks(translator, value, targetLanguage) {
  const pieces = splitOfflineTranslationText(value)
  const output = []
  for (const piece of pieces) {
    if (!piece.translatable) {
      output.push(piece.text)
      continue
    }
    const encoded = await translator.tokenizer(piece.text)
    translationDiagnostic('input-tokenized')
    const sourceIds = Array.from(encoded.input_ids.data, value => BigInt(value))
    const ids = prependSmall100TargetToken(sourceIds, targetLanguage, translator.languageTokens)
    const inputIds = new translator.Tensor('int64', BigInt64Array.from(ids), [1, ids.length])
    const attentionMask = new translator.Tensor('int64', BigInt64Array.from(ids, () => 1n), [1, ids.length])
    translationDiagnostic('generation-started')
    const result = await translator.model.generate({
      input_ids: inputIds,
      attention_mask: attentionMask,
      max_new_tokens: 512,
      num_beams: 1,
    })
    translationDiagnostic('generation-ready')
    output.push(decodeGenerated(translator.tokenizer, result))
  }
  return output.join('')
}

/**
 * @param {{ tokenizer: any, model: any, Tensor: any }} translator
 * @param {string} value
 * @param {{ model: string, targetLanguage: string }} leg
 */
async function translateOpusChunks(translator, value, leg) {
  const output = []
  for (const piece of splitOfflineTranslationText(value)) {
    if (!piece.translatable) {
      output.push(piece.text)
      continue
    }
    const encoded = await translator.tokenizer(piece.text)
    if (leg.model === 'opus-en-mul') selectOpusTarget(encoded, translator, leg.targetLanguage)
    const result = await translator.model.generate({
      ...encoded,
      max_new_tokens: 512,
      num_beams: 1,
    })
    output.push(decodeGenerated(translator.tokenizer, result))
  }
  return output.join('')
}

/** @param {any} tokenizer @param {any} result */
function decodeGenerated(tokenizer, result) {
  const generated = result?.[0]?.data
  const translated = generated
    ? tokenizer.decode(Array.from(generated, Number), { skip_special_tokens: true })
    : ''
  if (typeof translated !== 'string' || !translated.trim()) {
    throw new Error('The self-hosted translation engine returned an invalid response.')
  }
  return translated
}

/**
 * MarianTokenizer's fast-tokenizer compatibility path currently treats
 * `>>lang<<` as normal SentencePiece text. Insert the known vocabulary id
 * directly so the model receives the exact target selector it was trained on.
 *
 * @param {Iterable<bigint | number>} sourceIds
 * @param {string} targetLanguage
 * @param {Map<string, number> | Record<string, number>} vocabulary
 */
export function prependOpusTargetToken(sourceIds, targetLanguage, vocabulary) {
  const token = opusTargetTokens[targetLanguage]
  if (!token) throw new Error(`Unsupported OPUS translation target: ${targetLanguage || '<empty>'}.`)
  const key = `>>${token}<<`
  const targetId = vocabulary instanceof Map ? vocabulary.get(key) : vocabulary?.[key]
  if (typeof targetId !== 'number' || !Number.isSafeInteger(targetId) || targetId < 0) {
    throw new Error(`Unsupported OPUS translation target: ${targetLanguage || '<empty>'}.`)
  }
  return [BigInt(targetId), ...Array.from(sourceIds, value => BigInt(value))]
}

/** @param {any} encoded @param {{ tokenizer: any, Tensor: any }} translator @param {string} targetLanguage */
function selectOpusTarget(encoded, translator, targetLanguage) {
  const ids = prependOpusTargetToken(encoded.input_ids.data, targetLanguage, translator.tokenizer.get_vocab())
  encoded.input_ids = new translator.Tensor('int64', BigInt64Array.from(ids), [1, ids.length])
  encoded.attention_mask = new translator.Tensor('int64', BigInt64Array.from(ids, () => 1n), [1, ids.length])
}

/** @param {number} count @param {((loaded: number) => void) | undefined} onProgress */
function createProgressReporter(count, onProgress) {
  const values = Array.from({ length: count }, () => 0)
  let last = 0
  return {
    update(index, value) {
      values[index] = clampProgress(value)
      const next = count ? values.reduce((sum, current) => sum + current, 0) / count : 1
      last = Math.max(last, next)
      onProgress?.(last)
    },
  }
}

/** @param {string} modelName @param {((loaded: number) => void) | undefined} onProgress */
async function loadModel(modelName, onProgress) {
  const existing = modelPromises.get(modelName)
  if (existing) {
    const model = await existing
    onProgress?.(1)
    return model
  }

  const promise = createModel(modelName, onProgress).catch(error => {
    modelPromises.delete(modelName)
    translationDiagnostic(`error:${modelName}:${String(error?.message || error)}`)
    if (/not found locally|missing|404|local_files_only/i.test(String(error?.message || error))) {
      throw new Error('The self-hosted translation model is not installed on this CADUCEUS server.')
    }
    throw new Error('The self-hosted translation engine could not start.')
  })
  modelPromises.set(modelName, promise)
  return promise
}

/** @param {string} modelName @param {((loaded: number) => void) | undefined} onProgress */
async function createModel(modelName, onProgress) {
  const vendor = await loadVendor()
  if (modelName === 'small100') return loadSmall100Model(vendor, onProgress)
  if (modelName === 'opus-mul-en') {
    return loadOpusModel(vendor, OPUS_MULTILINGUAL_TO_ENGLISH_MODEL, OPUS_MULTILINGUAL_TO_ENGLISH_REVISION, onProgress)
  }
  if (modelName === 'opus-en-mul') {
    return loadOpusModel(vendor, OPUS_ENGLISH_TO_MULTILINGUAL_MODEL, OPUS_ENGLISH_TO_MULTILINGUAL_REVISION, onProgress)
  }
  throw new Error(`Unknown self-hosted translation model: ${modelName}.`)
}

async function loadVendor() {
  if (!vendorPromise) {
    vendorPromise = import('./translation-engine-vendor.js').then(async vendor => {
      translationDiagnostic('vendor-ready')
      const { env } = vendor
      env.allowLocalModels = true
      env.allowRemoteModels = false
      env.localModelPath = '/shared/assets/translation-models/'
      env.useBrowserCache = false
      env.fetch = createChunkedTranslationAssetFetch()
      const wasm = env.backends.onnx.wasm
      if (!wasm) throw new Error('ONNX Runtime WebAssembly backend is unavailable.')
      const response = await fetchSelfHostedTranslationAsset('runtime/ort-wasm-simd-threaded.asyncify.wasm')
      if (!response.ok) throw new Error('The self-hosted translation runtime could not be loaded.')
      translationDiagnostic('wasm-downloaded')
      wasm.wasmPaths = undefined
      wasm.wasmBinary = new Uint8Array(await response.arrayBuffer())
      wasm.proxy = false
      wasm.numThreads = 1
      translationDiagnostic('wasm-configured')
      return vendor
    }).catch(error => {
      vendorPromise = null
      throw error
    })
  }
  return vendorPromise
}

/** @param {any} vendor @param {((loaded: number) => void) | undefined} onProgress */
async function loadSmall100Model(vendor, onProgress) {
  const commonOptions = {
    local_files_only: true,
    revision: OFFLINE_TRANSLATION_MODEL_REVISION,
  }
  translationDiagnostic('model-load-started:small100')
  const [tokenizer, model, languageTokens] = await Promise.all([
    vendor.AutoTokenizer.from_pretrained(OFFLINE_TRANSLATION_MODEL, commonOptions),
    vendor.AutoModelForSeq2SeqLM.from_pretrained(OFFLINE_TRANSLATION_MODEL, {
      ...commonOptions,
      device: 'wasm',
      // These standard filenames contain int8-quantized weights.
      dtype: 'fp32',
      progress_callback: modelProgress(onProgress),
    }),
    loadLanguageTokens(),
  ])
  translationDiagnostic('model-ready:small100')
  onProgress?.(1)
  return { tokenizer, model, languageTokens, Tensor: vendor.Tensor }
}

/**
 * @param {any} vendor
 * @param {string} modelId
 * @param {string} revision
 * @param {((loaded: number) => void) | undefined} onProgress
 */
async function loadOpusModel(vendor, modelId, revision, onProgress) {
  const commonOptions = { local_files_only: true, revision }
  translationDiagnostic(`model-load-started:${modelId}`)
  const [tokenizer, model] = await Promise.all([
    vendor.AutoTokenizer.from_pretrained(modelId, commonOptions),
    vendor.AutoModelForSeq2SeqLM.from_pretrained(modelId, {
      ...commonOptions,
      device: 'wasm',
      dtype: 'int8',
      // ORT's QDQ optimizer currently rewrites Marian's tied embedding before
      // it discovers the corresponding scale. The original graph is valid and
      // executes correctly when this optional rewrite is disabled.
      session_options: { graphOptimizationLevel: 'disabled' },
      progress_callback: modelProgress(onProgress),
    }),
  ])
  translationDiagnostic(`model-ready:${modelId}`)
  onProgress?.(1)
  return { tokenizer, model, Tensor: vendor.Tensor }
}

/** @param {((loaded: number) => void) | undefined} onProgress */
function modelProgress(onProgress) {
  return event => {
    if (event?.status === 'progress_total') {
      onProgress?.(clampProgress(Number(event.progress || 0) / 100))
    }
  }
}

/** @param {string} language */
function assertSupportedLanguage(language) {
  if (!Object.hasOwn(TRANSLATION_LANGUAGES, language)) {
    throw new Error(`Unsupported self-hosted translation language: ${language || '<empty>'}.`)
  }
}

/**
 * SMaLL-100 selects the destination language by prepending the locked target
 * token to the source sequence. The encoder's existing EOS token is retained.
 *
 * @param {Iterable<bigint | number>} sourceIds
 * @param {string} targetLanguage
 * @param {Record<string, number>} languageTokens
 * @returns {bigint[]}
 */
export function prependSmall100TargetToken(sourceIds, targetLanguage, languageTokens) {
  const targetId = languageTokens?.[targetLanguage]
  if (!Number.isSafeInteger(targetId) || targetId < 0) {
    throw new Error(`Unsupported self-hosted translation target: ${targetLanguage || '<empty>'}.`)
  }
  return [BigInt(targetId), ...Array.from(sourceIds, value => BigInt(value))]
}

async function loadLanguageTokens() {
  const logicalPath = `models/${OFFLINE_TRANSLATION_MODEL}/lang_tokens.json`
  const response = await fetchSelfHostedTranslationAsset(logicalPath)
  if (!response.ok) throw new Error('The self-hosted translation language catalog could not be loaded.')
  const payload = await response.json()
  const tokens = payload?.lang_to_id
  if (!tokens || typeof tokens !== 'object' || Array.isArray(tokens)) {
    throw new Error('The self-hosted translation language catalog is invalid.')
  }
  for (const required of SMALL100_LANGUAGE_CODES) {
    if (!Number.isSafeInteger(tokens[required]) || tokens[required] < 0) {
      throw new Error(`The self-hosted translation language catalog is missing ${required}.`)
    }
  }
  return /** @type {Record<string, number>} */ (tokens)
}

/**
 * @param {string} value
 * @returns {Array<{ text: string, translatable: boolean }>}
 */
export function splitOfflineTranslationText(value) {
  const result = []
  for (const paragraph of String(value || '').split(/(\n+)/)) {
    if (!paragraph || /^\s+$/.test(paragraph)) {
      if (paragraph) result.push({ text: paragraph, translatable: false })
      continue
    }
    let remaining = paragraph
    while (remaining.length > OFFLINE_TRANSLATION_CHUNK_CHARACTERS) {
      const window = remaining.slice(0, OFFLINE_TRANSLATION_CHUNK_CHARACTERS + 1)
      let split = lastSentenceBoundary(window)
      if (split < OFFLINE_TRANSLATION_CHUNK_CHARACTERS / 3) split = window.lastIndexOf(' ', OFFLINE_TRANSLATION_CHUNK_CHARACTERS)
      if (split < 1) split = OFFLINE_TRANSLATION_CHUNK_CHARACTERS
      result.push({ text: remaining.slice(0, split), translatable: true })
      remaining = remaining.slice(split)
    }
    if (remaining) result.push({ text: remaining, translatable: true })
  }
  return result
}

/** @param {string} value */
function lastSentenceBoundary(value) {
  let boundary = -1
  for (const match of value.matchAll(/[.!?。！？]+\s+/gu)) boundary = match.index + match[0].length
  return boundary
}

/** @param {number} value */
function clampProgress(value) {
  return Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0))
}

/** @param {string} stage */
function translationDiagnostic(stage) {
  const observer = /** @type {any} */ (globalThis).__CADUCEUS_TRANSLATION_DIAGNOSTIC
  if (typeof observer === 'function') observer(stage)
}
