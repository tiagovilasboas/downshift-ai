// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { BelowSessionRow, EfficiencyRow } from '../../../types'
import { routingLines, routingSummary, skillsValue } from './normalize'

const row = (extra: Partial<BelowSessionRow> = {}): BelowSessionRow => ({
  harness: 'claude-code',
  observed: true,
  spawns: 192,
  belowSpawns: 150,
  billable: 25_000_000,
  belowBillable: 17_600_000,
  rewrittenSpawns: 46,
  rewrittenBillable: 5_300_000,
  fallbackSpawns: 0,
  fallbackBillable: 0,
  fallbackBelowBillable: 0,
  unobservedSpawns: 0,
  cacheRead: 0,
  ...extra,
})

const eff = (harness: string): EfficiencyRow => ({
  harness,
  spawns: 234,
  byTier: { small: 19, mid: 192, frontier: 23, unknown: 0 },
  ended: 0,
  completed: 0,
  reworked: 0,
  reworkRatio: 0,
  premiumObserved: false,
  premiumPct: 0,
  tierKnown: 0,
  nonFrontier: 0,
  tierUnknown: 0,
})

const text = (rows: BelowSessionRow[], e: EfficiencyRow[] = []) => routingLines(rows, e).map(l => `${l.label}: ${l.value}`)

test('one harness reads lighter model, session model, Downshift and tiers, in tokens', () => {
  expect(text([row()], [eff('claude-code')])).toEqual([
    'On a lighter model: 17.6M tokens · 150 of 192 agents',
    'On the session model: 7.4M tokens',
    'Moved by Downshift: 5.3M tokens · 46 agents',
    'Agents by tier · 30 days: small 19 · mid 192 · frontier 23',
  ])
})

test('the summary is the lighter-model share of one observed harness', () => {
  expect(routingSummary([row()], 150, 192)).toBe('70% of subagent tokens ran on a lighter model')
})

test('the session-model row is billable minus the lighter-model tokens', () => {
  const lines = routingLines([row({ billable: 1000, belowBillable: 250, rewrittenSpawns: 0 })])
  expect(lines.find(l => l.label === 'On the session model')?.value).toBe('750 tokens')
})

test('the Downshift row is hidden when it moved nothing', () => {
  const labels = routingLines([row({ rewrittenSpawns: 0, rewrittenBillable: 0 })]).map(l => l.label)
  expect(labels).not.toContain('Moved by Downshift')
  expect(labels).toContain('On a lighter model')
})

test('several harnesses get their own rows with the harness name and are never summed', () => {
  const rows = [row(), row({ harness: 'codex', spawns: 10, belowSpawns: 2, billable: 1000, belowBillable: 200, rewrittenSpawns: 0 })]
  const lines = text(rows)
  expect(lines).toContain('claude-code · On a lighter model: 17.6M tokens · 150 of 192 agents')
  expect(lines).toContain('codex · On a lighter model: 200 tokens · 2 of 10 agents')
  expect(routingSummary(rows, 152, 202)).toBe('152 of 202 agents on a lighter model')
})

test('nothing measured reads not measured yet, and no agent measured keeps its summary', () => {
  expect(routingLines([])).toEqual([{ label: 'Not measured yet', value: '', isObserved: false }])
  expect(routingSummary([], 0, 0)).toBe('no agent measured yet')
})

test('a harness placed against the requested model keeps its own labelled line', () => {
  const lines = text([row({ observed: false, fallbackSpawns: 1, fallbackBillable: 4000, fallbackBelowBillable: 1000 })])
  expect(lines).toEqual(['claude-code · no session model: 1.0k of 4.0k tokens lighter than asked (25%) · 1 agent'])
})

test('quotaShort renders only for observed harnesses and carries the harness prefix when several are observed', () => {
  const single = routingLines([row({ quotaShort: '41.0% lower (38M of 93M mid units)' })])
  expect(single.at(-1)).toEqual({ label: 'Quota estimate', value: '41.0% lower (38M of 93M mid units)', isObserved: true })

  const many = routingLines([
    row({ quotaShort: '41.0% lower (38M of 93M mid units)' }),
    row({ harness: 'codex', quotaShort: '12.0% lower (1M of 8M mid units)' }),
  ])
  expect(many.some(l => l.label === 'claude-code · Quota estimate' && l.value === '41.0% lower (38M of 93M mid units)' && l.isObserved)).toBe(true)
  expect(many.some(l => l.label === 'codex · Quota estimate' && l.value === '12.0% lower (1M of 8M mid units)' && l.isObserved)).toBe(true)

  const unobserved = routingLines([row({ observed: false, fallbackSpawns: 1, fallbackBillable: 4000, fallbackBelowBillable: 1000, quotaShort: 'ignored' })])
  expect(unobserved.some(l => l.label.includes('Quota estimate'))).toBe(false)
})

test('skillsValue formats positive integer counts sorted by key and hides empty or zero counts', () => {
  expect(skillsValue(undefined)).toBeUndefined()
  expect(skillsValue({})).toBeUndefined()
  expect(skillsValue({ ponytail: 0 })).toBeUndefined()
  expect(skillsValue({ ponytail: -1, bad: 1.5 })).toBeUndefined()
  expect(skillsValue({ zebra: 2, alpha: 1, ponytail: 3, zero: 0 })).toBe('alpha 1, ponytail 3, zebra 2')
})
