#!/usr/bin/env bun
/**
 * Restore `$`/`$$` prefixes in template *.html identifiers based on the
 * companion *.js class field declarations. Use case: after a blanket strip
 * of `$` from html (which broke prerender — Tachyon does NOT auto-prefix
 * template expressions), this re-prefixes any bare identifier in the
 * template that matches a `$`-prefixed (sessionStorage) or `$$`-prefixed
 * (localStorage) JS class field.
 *
 * Special cases preserved:
 *   - `$event` is left untouched (already correct).
 *   - In .html templates we only touch identifiers in attribute/value
 *     positions and inside `{…}` interpolation — never CSS or `${…}` JS
 *     template literals (those start with `${`, our regex excludes them).
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { resolve, dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { globSync } from 'node:fs'

const projectRoot = resolve(fileURLToPath(new URL('.', import.meta.url)), '..')

// Find every client/**/*.html
const htmlFiles = []
function walk(dir) {
  const { readdirSync, statSync } = require('node:fs')
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) walk(full)
    else if (full.endsWith('.html')) htmlFiles.push(full)
  }
}
walk(join(projectRoot, 'browser'))

let totalChanges = 0
for (const htmlPath of htmlFiles) {
  const jsPath = htmlPath.replace(/\.html$/, '.js')
  let jsSource = ''
  try { jsSource = readFileSync(jsPath, 'utf8') } catch { continue }

  // Parse class field declarations: `$foo = ...` and `$$foo = ...`
  // Only top-level (4-space indent or less); ignore matches inside strings.
  const dollarFields = new Set()
  const doubleDollarFields = new Set()
  // Crude but reliable: walk lines, match leading whitespace + identifier.
  for (const line of jsSource.split('\n')) {
    let m = line.match(/^\s+(\$\$[a-zA-Z_][a-zA-Z0-9_]*)\s*=/)
    if (m) { doubleDollarFields.add(m[1].slice(2)); continue }
    m = line.match(/^\s+(\$[a-zA-Z_][a-zA-Z0-9_]*)\s*=/)
    if (m) dollarFields.add(m[1].slice(1))
  }
  // Also catch getter forms: `get $foo()` or `$foo()` method-style
  for (const m of jsSource.matchAll(/\bget\s+(\$\$?)([a-zA-Z_][a-zA-Z0-9_]*)\s*\(/g)) {
    if (m[1] === '$$') doubleDollarFields.add(m[2])
    else dollarFields.add(m[2])
  }

  if (dollarFields.size === 0 && doubleDollarFields.size === 0) continue

  let html = readFileSync(htmlPath, 'utf8')
  let fileChanges = 0

  // Replace each bare identifier with its prefixed form.
  // Word-boundary on both sides. NOT preceded by `$` (already prefixed) or
  // a letter/digit (part of a longer ident) — and NOT preceded by `${`
  // (JS template literal).
  for (const name of doubleDollarFields) {
    const re = new RegExp(`(?<![\\w$])(?<!\\$\\{)${name}(?!\\w)`, 'g')
    const before = html
    html = html.replace(re, '$$$$' + name) // $$ in replacement → literal $$
    if (html !== before) fileChanges += (html.length - before.length) / 2
  }
  for (const name of dollarFields) {
    const re = new RegExp(`(?<![\\w$])(?<!\\$\\{)${name}(?!\\w)`, 'g')
    const before = html
    html = html.replace(re, '$$' + name) // $$ in replacement → literal $
    if (html !== before) fileChanges += html.length - before.length
  }

  if (fileChanges > 0) {
    writeFileSync(htmlPath, html)
    totalChanges += fileChanges
    console.log(`[restore] ${htmlPath.replace(projectRoot, '.')}: +${fileChanges} chars`)
  }
}

console.log(`Done. Total ${totalChanges} char changes across ${htmlFiles.length} files.`)
