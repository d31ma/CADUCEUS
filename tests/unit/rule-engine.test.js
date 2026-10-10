import { describe, expect, it } from 'bun:test'
import { evaluateInboxRule } from '@/features/mailbox/services/rule-engine.js'

const message = {
  from: 'Billing <billing@vendor.example>',
  to: ['alice@example.com'],
  cc: ['finance@example.com'],
  subject: 'August Invoice',
  body: 'The total is attached.',
  hasAttachment: true,
}

describe('mailbox rule evaluator', () => {
  it('uses all richer fields and returns a side-effect-free action plan', () => {
    const rule = {
      conditionMatch: 'all',
      stopProcessing: true,
      conditions: [
        { field: 'from', op: 'endsWith', value: 'vendor.example>' },
        { field: 'cc', op: 'contains', value: 'finance@' },
        { field: 'subject', op: 'notEquals', value: 'July Invoice' },
        { field: 'body', op: 'notContains', value: 'overdue' },
        { field: 'hasAttachment', op: 'equals', value: 'true' },
      ],
      actions: [{ type: 'folder', folder: 'finance' }, { type: 'star', value: true }],
    }
    const result = evaluateInboxRule(rule, message)
    expect(result.matched).toBe(true)
    expect(result.conditionResults.every(item => item.matched)).toBe(true)
    expect(result.plannedActions).toEqual(rule.actions)
    expect(result.plannedActions).not.toBe(rule.actions)
    expect(result.stopProcessing).toBe(true)
  })

  it('supports any and withholds actions for a miss', () => {
    const result = evaluateInboxRule({
      conditionMatch: 'any',
      conditions: [{ field: 'subject', op: 'equals', value: 'no' }, { field: 'hasAttachment', op: 'notEquals', value: 'true' }],
      actions: [{ type: 'delete' }],
    }, message)
    expect(result.matched).toBe(false)
    expect(result.plannedActions).toEqual([])
    expect(result.stopProcessing).toBe(false)
  })
})
