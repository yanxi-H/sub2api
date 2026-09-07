export interface NumberSummary {
  p95: number
  p90: number
  p50: number
  avg: number
  max: number
}

function percentile(sortedValues: number[], quantile: number): number {
  if (!sortedValues.length) return 0
  const position = (sortedValues.length - 1) * quantile
  const lowerIndex = Math.floor(position)
  const upperIndex = Math.ceil(position)
  const weight = position - lowerIndex
  return sortedValues[lowerIndex] * (1 - weight) + sortedValues[upperIndex] * weight
}

export function summarizeNumbers(values: number[]): NumberSummary {
  const safeValues = values.filter(Number.isFinite).sort((a, b) => a - b)
  if (!safeValues.length) return { p95: 0, p90: 0, p50: 0, avg: 0, max: 0 }

  const total = safeValues.reduce((sum, value) => sum + value, 0)
  return {
    p95: percentile(safeValues, 0.95),
    p90: percentile(safeValues, 0.9),
    p50: percentile(safeValues, 0.5),
    avg: total / safeValues.length,
    max: safeValues[safeValues.length - 1]
  }
}
