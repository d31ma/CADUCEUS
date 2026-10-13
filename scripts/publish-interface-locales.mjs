#!/usr/bin/env bun

import { createHash } from 'node:crypto'
import { readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import OpenCC from 'opencc-js'
import reviewedInterfaceTranslations from './data/interface-locale-overrides.mjs'
import ar from '../client/shared/locales/ar.js'
import en from '../client/shared/locales/en.js'
import fr from '../client/shared/locales/fr.js'

const root = resolve(import.meta.dir, '..')
const outputPath = resolve(root, 'client/shared/assets/interface-locales.json')
const sourceModel = 'casawolice/small100-onnx'
const sourceRevision = '5c2c73ac70bee9c58f5a7ac5e84a36bee25db8ee'
const generationProfile = 'sentence-boundary-greedy-v1'
const reviewProfile = 'mail-core-v1'
const postProcessors = Object.freeze({ 'zh-TW': 'opencc-js@1.4.1 cn-to-tw' })
const locales = Object.freeze([
  'ar', 'cs', 'de', 'es', 'es-419', 'fr', 'gl', 'he', 'hu', 'ja', 'ko', 'pl',
  'pt', 'pt-BR', 'ru', 'tr', 'uk', 'zh-CN', 'zh-TW',
])
const localeSet = new Set(locales)
const englishIds = new Map()
for (const [id, value] of Object.entries(en)) {
  if (typeof value === 'string') englishIds.set(value, id)
}
const curatedCatalogs = { ar, fr }
const toTaiwanTraditional = OpenCC.Converter({ from: 'cn', to: 'tw' })

if (import.meta.main) main()

function main() {
  const mode = process.argv[2] || '--check'
  if (mode === '--check') {
    const asset = JSON.parse(readFileSync(outputPath, 'utf8'))
    validatePublishedAsset(asset)
    console.log(`Interface locale asset verified: ${asset.inventory.length} literals × ${locales.length} locales (${asset.sha256}).`)
  } else if (mode === '--publish') {
    const inputs = process.argv.slice(3)
    if (!inputs.length) throw new Error('Usage: bun scripts/publish-interface-locales.mjs --publish RAW.json [...]')
    const asset = publish(inputs)
    writeFileSync(outputPath, `${JSON.stringify(asset)}\n`)
    console.log(`Published ${asset.inventory.length} interface literals × ${locales.length} locales to ${outputPath}.`)
    for (const locale of locales) console.log(`${locale}: ${JSON.stringify(asset.quality[locale])}`)
  } else {
    throw new Error('Usage: bun scripts/publish-interface-locales.mjs [--check|--publish RAW.json [...]]')
  }
}

function publish(inputs) {
  const documents = inputs.map(path => JSON.parse(readFileSync(resolve(path), 'utf8')))
  const inventory = validateRawDocuments(documents)
  const rawCatalogs = Object.assign({}, ...documents.map(document => document.catalogs))
  const catalogs = {}
  const quality = {}
  for (const locale of locales) {
    const stats = { curated: 0, generated: 0, fallback: 0 }
    const catalog = {}
    const generatedSources = new Set()
    for (const source of inventory) {
      const curated = curatedTranslation(source, locale)
      if (curated) {
        catalog[source] = curated
        stats.curated += 1
        continue
      }
      const raw = locale === 'zh-TW'
        ? toTaiwanTraditional(rawCatalogs['zh-CN']?.[source] || '')
        : rawCatalogs[locale]?.[source]
      const generated = safeGeneratedTranslation(source, raw, locale)
      if (generated) {
        catalog[source] = generated
        generatedSources.add(source)
        stats.generated += 1
      } else {
        catalog[source] = source
        stats.fallback += 1
      }
    }
    removeCollapsedTranslations(catalog, generatedSources, stats)
    catalogs[locale] = catalog
    quality[locale] = stats
    assertSemanticSentinels(locale, catalog)
  }
  const payload = { inventory, catalogs }
  return {
    schemaVersion: 1,
    sourceModel,
    sourceRevision,
    generationProfile,
    reviewProfile,
    postProcessors,
    sha256: digest(payload),
    ...payload,
    quality,
  }
}

function removeCollapsedTranslations(catalog, generatedSources, stats) {
  const owners = new Map()
  for (const [source, target] of Object.entries(catalog)) {
    if (source === target) continue
    const sources = owners.get(target) || []
    sources.push(source)
    owners.set(target, sources)
  }
  for (const sources of owners.values()) {
    if (sources.length < 4) continue
    for (const source of sources) {
      if (!generatedSources.has(source)) continue
      catalog[source] = source
      stats.generated -= 1
      stats.fallback += 1
    }
  }
}

function assertSemanticSentinels(locale, catalog) {
  const sources = ['Account settings', 'Actions', 'Activity', 'Appearance', 'Archive']
  const values = sources.map(source => catalog[source]).filter(Boolean)
  if (values.length === sources.length && new Set(values).size < 3) {
    throw new Error(`Generated ${locale} catalog collapsed distinct interface rows.`)
  }
  const owners = new Map()
  for (const [source, target] of Object.entries(catalog)) {
    if (source === target) continue
    const sourcesForTarget = owners.get(target) || []
    sourcesForTarget.push(source)
    owners.set(target, sourcesForTarget)
  }
  const collapsed = [...owners.entries()].find(([, sourceValues]) => sourceValues.length >= 4)
  if (collapsed) throw new Error(`Generated ${locale} catalog maps unrelated literals to ${JSON.stringify(collapsed[0])}.`)
}

function validateRawDocuments(documents) {
  const inventory = documents[0]?.inventory
  if (!Array.isArray(inventory) || inventory.length < 500 || !inventory.every(isNonEmptyString)) {
    throw new Error('Raw interface inventory is missing, unexpectedly small, or malformed.')
  }
  const encoded = JSON.stringify(inventory)
  const seen = new Set()
  for (const document of documents) {
    if (document?.schemaVersion !== 1 || JSON.stringify(document.inventory) !== encoded) {
      throw new Error('Raw interface catalogs do not share one schema and exact inventory.')
    }
    for (const [locale, catalog] of Object.entries(document.catalogs || {})) {
      if (!localeSet.has(locale) || seen.has(locale)) throw new Error(`Unexpected or duplicate raw locale: ${locale}.`)
      if (!catalog || Object.keys(catalog).length !== inventory.length) throw new Error(`Incomplete raw locale: ${locale}.`)
      seen.add(locale)
    }
  }
  if (seen.size !== locales.length || locales.some(locale => !seen.has(locale))) {
    throw new Error(`Raw catalogs must contain exactly: ${locales.join(', ')}.`)
  }
  return inventory
}

function validatePublishedAsset(asset) {
  if (asset?.schemaVersion !== 1 || !Array.isArray(asset.inventory) || asset.inventory.length < 500) {
    throw new Error('Published interface locale asset has an invalid schema or inventory.')
  }
  if (asset.sourceModel !== sourceModel || asset.sourceRevision !== sourceRevision || asset.generationProfile !== generationProfile || asset.reviewProfile !== reviewProfile ||
      JSON.stringify(asset.postProcessors) !== JSON.stringify(postProcessors)) {
    throw new Error('Published interface locale provenance does not match the qualified pipeline.')
  }
  const keys = Object.keys(asset.catalogs || {}).sort()
  if (JSON.stringify(keys) !== JSON.stringify([...locales].sort())) throw new Error('Published interface locale set is incomplete.')
  const inventorySet = new Set(asset.inventory)
  if (inventorySet.size !== asset.inventory.length || !asset.inventory.every(isNonEmptyString)) {
    throw new Error('Published interface inventory contains duplicates or invalid values.')
  }
  for (const locale of locales) {
    const catalog = asset.catalogs[locale]
    if (!catalog || Object.keys(catalog).length !== asset.inventory.length) throw new Error(`Published ${locale} catalog is incomplete.`)
    for (const source of asset.inventory) {
      if (!Object.hasOwn(catalog, source) || !isNonEmptyString(catalog[source])) throw new Error(`Published ${locale} catalog omits ${JSON.stringify(source)}.`)
    }
    const stats = asset.quality?.[locale]
    if (!stats || !['curated', 'generated', 'fallback'].every(key => Number.isSafeInteger(stats[key]) && stats[key] >= 0) ||
        stats.curated + stats.generated + stats.fallback !== asset.inventory.length ||
        stats.fallback > Math.ceil(asset.inventory.length * 0.25)) {
      throw new Error(`Published ${locale} catalog does not meet the qualified translation coverage.`)
    }
    assertSemanticSentinels(locale, catalog)
  }
  if (asset.sha256 !== digest({ inventory: asset.inventory, catalogs: asset.catalogs })) {
    throw new Error('Published interface locale digest does not match its payload.')
  }
}

function curatedTranslation(source, locale) {
  const reviewed = reviewedInterfaceTranslations[locale]?.[source]
  if (typeof reviewed === 'string' && reviewed.trim()) return reviewed.trim()
  const id = englishIds.get(source)
  const value = id ? curatedCatalogs[locale]?.[id] : undefined
  return typeof value === 'string' && value.trim() ? value.trim() : ''
}

function safeGeneratedTranslation(source, candidate, locale) {
  const target = firstTranslation(source, candidate)
  if (!target || target === source || hasUnsafeMarker(target) || !preservesPlaceholders(source, target)) return ''
  const sourceWords = wordCount(source)
  const targetWords = wordCount(target)
  const shortLimit = source.length < 20 ? sourceWords * 2 : sourceWords * 2.5 + 4
  if (targetWords > Math.max(2, shortLimit) || target.length > Math.max(32, source.length * 2.5)) return ''
  if (wordCount(source) >= 1 && /^[\p{Script=Latin}]{1,2}$/u.test(target) && source.length >= 4) return ''
  if (hasRepeatedRun(target) || weakLexicalDiversity(target)) return ''
  if (!hasExpectedScript(target, locale)) return ''
  return target
}

function firstTranslation(source, candidate) {
  const normalized = String(candidate || '').replace(/\s+/g, ' ').trim()
  const completed = /[.!?。！？]\s*$/u.test(normalized)
  const firstSentence = normalized.match(/^.*?[.!?。！？](?=\s|$)/u)?.[0]
  const completeCandidate = /[.!?]\s+\S/u.test(source) ? normalized : firstSentence || normalized
  const completeValue = completeCandidate.replace(/[.!?…。！？]+\s*$/u, '').trim()
  const completeLimit = Math.max(4, wordCount(source) * 2 + 2)
  if (completed && wordCount(completeValue) <= completeLimit) {
    return restoreTerminalPunctuation(source, completeValue)
  }
  const segment = normalized.split(/\s*(?:\.{2,}|…|→|•|>>|<<)\s*/u, 1)[0] || ''
  const words = segment.split(/\s+/u).filter(Boolean)
  const sourceWords = wordCount(source)
  const limit = sourceWords <= 1 ? 1 : sourceWords + 1
  const value = words.slice(0, limit).join(' ').replace(/^[,;:|]+|[,;:|]+$/gu, '').trim()
  return restoreTerminalPunctuation(source, value)
}

function restoreTerminalPunctuation(source, target) {
  const punctuation = source.match(/[.!?…:]$/u)?.[0] || ''
  return `${target.replace(/[.!?…。！？]+$/u, '').trim()}${punctuation}`
}

function hasUnsafeMarker(value) {
  return />>|<<|<script|\[UNK\]|\b(?:undefined|null)\b|\s\+\s|=>|\|\||&&/iu.test(value)
}

function preservesPlaceholders(source, target) {
  const pattern = /\{[A-Za-z][A-Za-z0-9_]*\}/g
  return JSON.stringify((source.match(pattern) || []).sort()) === JSON.stringify((target.match(pattern) || []).sort())
}

function hasRepeatedRun(value) {
  return /(?:^|\s)([^\s]{2,})(?:\s+\1){2,}(?:\s|$)/iu.test(value)
}

function weakLexicalDiversity(value) {
  const words = value.toLocaleLowerCase().match(/[\p{L}\p{N}]+/gu) || []
  return words.length >= 8 && new Set(words).size / words.length < 0.45
}

function hasExpectedScript(value, locale) {
  const letters = value.match(/\p{L}/gu) || []
  if (letters.length < 3) return true
  const patterns = {
    ar: /\p{Script=Arabic}/u,
    he: /\p{Script=Hebrew}/u,
    ja: /[\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Han}]/u,
    ko: /\p{Script=Hangul}/u,
    ru: /\p{Script=Cyrillic}/u,
    uk: /\p{Script=Cyrillic}/u,
    'zh-CN': /\p{Script=Han}/u,
    'zh-TW': /\p{Script=Han}/u,
  }
  const pattern = patterns[locale]
  if (!pattern) return true
  return letters.filter(letter => pattern.test(letter)).length / letters.length >= 0.45
}

function wordCount(value) {
  return (String(value).match(/[\p{L}\p{N}]+/gu) || []).length
}

function isNonEmptyString(value) {
  return typeof value === 'string' && Boolean(value.trim())
}

function digest(value) {
  return createHash('sha256').update(JSON.stringify(value)).digest('hex')
}

export { firstTranslation, safeGeneratedTranslation, validatePublishedAsset }
