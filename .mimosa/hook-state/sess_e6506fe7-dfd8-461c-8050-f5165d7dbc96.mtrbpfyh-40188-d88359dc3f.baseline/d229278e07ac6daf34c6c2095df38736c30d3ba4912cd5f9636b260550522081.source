import { describe, expect, it, vi } from 'vitest'
import type { ChartEvent, ChartType, LegendElement, LegendItem } from 'chart.js'
import { focusOrAccumulateLegendClick } from '../chartLegendSelection'

function createDatasetLegend(count: number) {
  const visibility = Array.from({ length: count }, () => true)
  const items: LegendItem[] = visibility.map((_, datasetIndex) => ({
    text: `series-${datasetIndex}`,
    datasetIndex
  }))
  const chart = {
    isDatasetVisible: (index: number) => visibility[index],
    setDatasetVisibility: (index: number, visible: boolean) => { visibility[index] = visible },
    getDataVisibility: vi.fn(),
    toggleDataVisibility: vi.fn(),
    update: vi.fn()
  }
  const legend = { chart, legendItems: items } as unknown as LegendElement<ChartType>
  const click = (index: number) => focusOrAccumulateLegendClick({} as ChartEvent, items[index], legend)
  return { visibility, chart, click }
}

function createDataLegend(count: number) {
  const visibility = Array.from({ length: count }, () => true)
  const items: LegendItem[] = visibility.map((_, index) => ({
    text: `item-${index}`,
    index
  }))
  const chart = {
    isDatasetVisible: vi.fn(),
    setDatasetVisibility: vi.fn(),
    getDataVisibility: (index: number) => visibility[index],
    toggleDataVisibility: (index: number) => { visibility[index] = !visibility[index] },
    update: vi.fn()
  }
  const legend = { chart, legendItems: items } as unknown as LegendElement<ChartType>
  const click = (index: number) => focusOrAccumulateLegendClick(
    {} as ChartEvent,
    { ...items[index] },
    legend
  )
  return { visibility, click }
}

describe('focusOrAccumulateLegendClick', () => {
  it('focuses the first clicked series, accumulates later series, and restores all', () => {
    const { visibility, chart, click } = createDatasetLegend(3)

    click(1)
    expect(visibility).toEqual([false, true, false])

    click(2)
    expect(visibility).toEqual([false, true, true])

    click(0)
    expect(visibility).toEqual([true, true, true])
    expect(chart.update).toHaveBeenCalledTimes(3)
  })

  it('restores all series when the sole focused series is clicked again', () => {
    const { visibility, click } = createDatasetLegend(3)

    click(0)
    expect(visibility).toEqual([true, false, false])

    click(0)
    expect(visibility).toEqual([true, true, true])
  })

  it('removes a selected series in multi-select mode and restores instead of becoming empty', () => {
    const { visibility, click } = createDatasetLegend(3)

    click(0)
    click(1)
    click(0)
    expect(visibility).toEqual([false, true, false])

    click(1)
    expect(visibility).toEqual([true, true, true])
  })

  it('uses the same focus and restore interaction for data-item legends', () => {
    const { visibility, click } = createDataLegend(3)

    click(1)
    expect(visibility).toEqual([false, true, false])

    click(1)
    expect(visibility).toEqual([true, true, true])
  })
})
