#!/usr/bin/env bun
/**
 * CRAP score = CC^2 * (1 - coverage)^3 + CC
 *
 * Usage:
 *   bun test <suites> --coverage --coverage-reporter=lcov
 *   bun scripts/crap.mjs [--lcov coverage/lcov.info] [--max 15] [dir ...]
 *
 * ponytail: functions nested inside another function (callbacks, arrows) are
 * folded into the enclosing function rather than scored separately -- same rule
 * Java-based CRAP tools use for lambdas. Split them out if that ever misleads.
 */
import { parse } from 'acorn'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

const args = process.argv.slice(2)
const opt = (name, fallback) => {
  const i = args.indexOf(`--${name}`)
  return i === -1 ? fallback : args[i + 1]
}
const LCOV = opt('lcov', 'coverage/lcov.info')
const MAX = Number(opt('max', 15))
const ROOTS = args.filter((a, i) => !a.startsWith('--') && !args[i - 1]?.startsWith('--'))
const DIRS = ROOTS.length ? ROOTS : ['server', 'db']

/** @returns {Map<string, Map<number, number>>} file -> line -> hits */
function parseLcov(path) {
  const files = new Map()
  /** @type {string | null} */
  let cur = null
  for (const line of readFileSync(path, 'utf8').split('\n')) {
    if (line.startsWith('SF:')) files.set((cur = line.slice(3).trim()), new Map())
    else if (line.startsWith('DA:') && cur) {
      const [ln, hits] = line.slice(3).split(',').map(Number)
      files.get(cur).set(ln, hits)
    }
  }
  return files
}

function* walkFiles(dir) {
  for (const entry of readdirSync(dir)) {
    // server/deps is vendored (gitignored) — not our code, not our score.
    if (entry === 'node_modules' || entry === 'deps' || entry.startsWith('.')) continue
    const p = join(dir, entry)
    if (statSync(p).isDirectory()) yield* walkFiles(p)
    else if (/\.m?js$/.test(entry)) yield p
  }
}

const isFn = (node) => ['FunctionDeclaration', 'FunctionExpression', 'ArrowFunctionExpression'].includes(node?.type)

/** @type {WeakMap<object, any>} */
const parents = new WeakMap()

/** Visit Acorn child nodes without depending on TypeScript's compiler API. */
function forEachChild(node, callback) {
  for (const [key, value] of Object.entries(node)) {
    if (['start', 'end', 'loc', 'range'].includes(key)) continue
    if (Array.isArray(value)) {
      for (const child of value) {
        if (child && typeof child === 'object' && typeof child.type === 'string') {
          parents.set(child, node)
          callback(child)
        }
      }
    } else if (value && typeof value === 'object' && typeof value.type === 'string') {
      parents.set(value, node)
      callback(value)
    }
  }
}

function complexity(node) {
  let cc = 1
  const visit = (n) => {
    if (['IfStatement', 'ConditionalExpression', 'ForStatement', 'ForInStatement', 'ForOfStatement',
      'WhileStatement', 'DoWhileStatement', 'CatchClause'].includes(n.type)) cc++
    else if (n.type === 'SwitchCase' && n.consequent.length) cc++
    else if (n.type === 'AssignmentExpression') {
      // ponytail: ESLint `complexity` convention -- logical assignment counts,
      // plain && / || / ?? do not. Counting them would reward deleting `??`
      // defaults, which makes the code worse, not simpler.
      if (['&&=', '||=', '??='].includes(n.operator)) cc++
    }
    forEachChild(n, visit)
  }
  forEachChild(node, visit)
  return cc
}

function propertyName(node) {
  if (!node) return null
  if (node.type === 'Identifier' || node.type === 'PrivateIdentifier') return node.name
  if (node.type === 'Literal') return String(node.value)
  return null
}

function nameOf(node) {
  if (node.id?.name) return node.id.name
  const p = parents.get(node)
  if (p?.type === 'VariableDeclarator') return propertyName(p.id) || '<anonymous>'
  if (['Property', 'PropertyDefinition', 'MethodDefinition'].includes(p?.type)) return propertyName(p.key) || '<anonymous>'
  return '<anonymous>'
}

function classOf(node) {
  for (let p = parents.get(node); p; p = parents.get(p))
    if (p.type === 'ClassDeclaration' || p.type === 'ClassExpression') return p.id?.name ?? '<class>'
  return null
}

const cov = parseLcov(LCOV)
const rows = []

for (const dir of DIRS) {
  for (const file of walkFiles(dir)) {
    const rel = relative(process.cwd(), file)
    const src = readFileSync(file, 'utf8')
    // Tachyon route decorators are line-oriented. Blank them without changing
    // line numbers so Acorn can parse the remaining standard JavaScript.
    const parseable = src.replace(/^\s*@[A-Za-z_$][^\n]*$/gm, (line) => ' '.repeat(line.length))
    const sf = parse(parseable, { ecmaVersion: 'latest', sourceType: 'module', locations: true, allowHashBang: true })
    const lines = cov.get(rel) ?? cov.get(`./${rel}`) ?? new Map()

    const collect = (node, inFn) => {
      if (isFn(node) && !inFn) {
        const start = node.loc.start.line
        const end = node.loc.end.line
        let total = 0, hit = 0
        for (let l = start; l <= end; l++) {
          if (!lines.has(l)) continue
          total++
          if (lines.get(l) > 0) hit++
        }
        const coverage = total ? hit / total : 0
        const cc = complexity(node)
        const cls = classOf(node)
        rows.push({
          file: rel,
          line: start,
          name: (cls ? `${cls}.` : '') + nameOf(node),
          cc,
          coverage,
          lines: total,
          untested: total === 0,
          crap: cc * cc * (1 - coverage) ** 3 + cc,
        })
        forEachChild(node, (c) => collect(c, true))
        return
      }
      forEachChild(node, (c) => collect(c, inFn))
    }
    collect(sf, false)
  }
}

rows.sort((a, b) => b.crap - a.crap)

const over = rows.filter((r) => r.crap > MAX)
const pct = (n) => `${(n * 100).toFixed(0)}%`
console.log(`\nCRAP report  (${rows.length} functions in ${DIRS.join(', ')}, threshold ${MAX})\n`)
console.log(`  over threshold : ${over.length} (${pct(over.length / rows.length)})`)
console.log(`  worst          : ${rows[0]?.crap.toFixed(1) ?? '-'}`)
console.log(`  median         : ${rows.length ? rows[Math.floor(rows.length / 2)].crap.toFixed(1) : '-'}`)
console.log(`  total CC       : ${rows.reduce((s, r) => s + r.cc, 0)}\n`)
console.log(`  never loaded   : ${rows.filter((r) => r.untested).length} functions (no coverage data at all)\n`)
console.log('  CRAP    CC   COV  LINES  FUNCTION')
for (const r of over.slice(0, Number(opt('top', 40)))) {
  console.log(
    `  ${r.crap.toFixed(1).padStart(6)}  ${String(r.cc).padStart(3)}  ${(r.untested ? '-' : pct(r.coverage)).padStart(4)}  ${String(r.lines).padStart(5)}  ${r.name}  ${r.file}:${r.line}`,
  )
}
if (args.includes('--json')) console.log('\n' + JSON.stringify(rows, null, 2))
process.exit(over.length ? 1 : 0)
