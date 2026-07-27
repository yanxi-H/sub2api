import { describe, expect, it } from 'vitest'
import { summarizeNumbers } from '../numberSummary'

describe('summarizeNumbers', () => {
  it('calculates interpolated percentiles, average, and maximum', () => {
    expect(summarizeNumbers([0, 1, 2, 3, 4])).toEqual({
      p95: 3.8,
      p90: 3.6,
      p50: 2,
      avg: 2,
      max: 4
    })
  })

  it('ignores non-finite values and returns zeros for empty input', () => {
    expect(summarizeNumbers([Number.NaN, Number.POSITIVE_INFINITY])).toEqual({
      p95: 0,
      p90: 0,
      p50: 0,
      avg: 0,
      max: 0
    })
  })
})
