#!/usr/bin/env bun
/**
 * Audit OPTIONS.schema.json coverage.
 *
 * For every route method, computes the set of HTTP statuses it can actually
 * return — from its guard decorator (auth, webhook, testOnly) plus every
 * r4xx/r422/r429 helper in its body — and compares against the statuses
 * declared in the route's OPTIONS.schema.json. Any status that a handler can
 * emit but the schema doesn't declare would leak out as HTTP 200 (Tachyon's
 * matchStatusCode falls through to 200 when nothing matches).
 *
 *   bun scripts/audit-response-schemas.mjs           # report gaps
 *   bun scripts/audit-response-schemas.mjs --fix     # write missing statuses
 */

import { readFileSync, writeFileSync, existsSync } from 'fs'
import { Glob } from 'bun'

const FIX = process.argv.includes('--fix')
const ROUTES = 'server/routes'

/** marker helper → [status, field-schema] */
const MARKER = {
  r400: ['400', { error: '^.*$' }],
  r401: ['401', { error: '^.*$', unauthorized: '^(true|false)$' }],
  r403: ['403', { error: '^.*$', forbidden: '^(true|false)$' }],
  r404: ['404', { error: '^.*$', notFound: '^(true|false)$' }],
  r422: ['422', { error: '^.*$', blocked: ['^.*$'] }],
  r429: ['429', { error: '^.*$', retryAfterSeconds: '^\\d+$', rateLimited: '^(true|false)$' }],
}

/** @param {string} decoratorLine @returns {string[]} statuses the guard can emit */
function decoratorStatuses(decoratorLine) {
  if (/@webhookSignature|@webhookSecret/.test(decoratorLine)) return ['401']
  if (/@testOnly/.test(decoratorLine)) return ['403']
  if (/@auth\(/.test(decoratorLine)) {
    // role or elevated level can also 403
    return /role:|level:\s*'(sensitive|credential)'/.test(decoratorLine) ? ['401', '403'] : ['401']
  }
  return []
}

function matchBrace(s, open) {
  let d = 0
  for (let i = open; i < s.length; i++) {
    if (s[i] === '{') d++
    else if (s[i] === '}') { d--; if (d === 0) return i }
  }
  return -1
}

const gaps = []
let routesScanned = 0

for (const file of new Glob(`${ROUTES}/**/yon.js`).scanSync()) {
  const src = readFileSync(file, 'utf8')
  const routeDir = file.slice(0, -'/yon.js'.length)
  const route = routeDir.slice(ROUTES.length + 1)
  const schemaPath = `${routeDir}/OPTIONS.schema.json`
  const schema = existsSync(schemaPath) ? JSON.parse(readFileSync(schemaPath, 'utf8')) : {}
  routesScanned++

  // Walk each static method
  const re = /([ \t]*)(?:(@\w+\([^)]*\)|@\w+)\s*\n[ \t]*)?static\s+async\s+(GET|POST|PUT|DELETE|PATCH)\s*\(/g
  let m
  while ((m = re.exec(src))) {
    const decorator = m[2] ?? ''
    const method = m[3]
    const braceOpen = src.indexOf('{', re.lastIndex)
    const braceClose = matchBrace(src, braceOpen)
    const body = src.slice(braceOpen, braceClose < 0 ? undefined : braceClose)

    const required = new Set(decoratorStatuses(decorator))
    for (const helper of Object.keys(MARKER)) {
      if (new RegExp(`\\b${helper}\\(`).test(body)) required.add(MARKER[helper][0])
    }
    if (required.size === 0) continue

    const declared = new Set(Object.keys(schema[method] ?? {}))
    const missing = [...required].filter(s => !declared.has(s)).sort()
    if (missing.length) gaps.push({ route, method, missing, schemaPath, hasFile: existsSync(schemaPath) })
  }
}

if (gaps.length === 0) {
  console.log(`✓ ${routesScanned} routes scanned — every emittable status is declared.`)
  process.exit(0)
}

console.log(`Found ${gaps.length} method(s) with undeclared statuses:\n`)
for (const g of gaps) {
  console.log(`  ${g.hasFile ? ' ' : '+'} ${g.route}  ${g.method}  missing: ${g.missing.join(', ')}${g.hasFile ? '' : '   (no schema file)'}`)
}

if (!FIX) {
  console.log(`\nRun with --fix to add the missing statuses.`)
  process.exit(1)
}

// ── Apply fixes: merge missing statuses into each route schema ──
const byPath = new Map()
for (const g of gaps) {
  if (!byPath.has(g.schemaPath)) byPath.set(g.schemaPath, [])
  byPath.get(g.schemaPath).push(g)
}
for (const [path, entries] of byPath) {
  const schema = existsSync(path) ? JSON.parse(readFileSync(path, 'utf8')) : {}
  for (const { method, missing } of entries) {
    schema[method] ??= {}
    for (const status of missing) {
      const match = Object.values(MARKER).find(([code]) => code === status)
      if (!match) continue
      const [, fields] = match
      schema[method][status] = fields
    }
    // keep statuses ordered ascending for readability
    schema[method] = Object.fromEntries(Object.entries(schema[method]).sort((a, b) => Number(a[0]) - Number(b[0])))
  }
  writeFileSync(path, JSON.stringify(schema, null, 2) + '\n')
  console.log(`  wrote ${path}`)
}
console.log(`\n✓ Fixed ${gaps.length} method(s) across ${byPath.size} schema file(s).`)
