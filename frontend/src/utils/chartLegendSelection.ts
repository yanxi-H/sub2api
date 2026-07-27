import {
  Chart,
  Legend,
  type ChartEvent,
  type ChartType,
  type LegendElement,
  type LegendItem
} from 'chart.js'

function itemVisible(legend: LegendElement<ChartType>, item: LegendItem): boolean {
  if (typeof item.datasetIndex === 'number') {
    return legend.chart.isDatasetVisible(item.datasetIndex)
  }
  if (typeof item.index === 'number') {
    return legend.chart.getDataVisibility(item.index)
  }
  return false
}

function sameItem(left: LegendItem, right: LegendItem): boolean {
  if (typeof left.datasetIndex === 'number' || typeof right.datasetIndex === 'number') {
    return left.datasetIndex === right.datasetIndex
  }
  return typeof left.index === 'number' && left.index === right.index
}

function setItemVisible(
  legend: LegendElement<ChartType>,
  item: LegendItem,
  visible: boolean
): void {
  if (typeof item.datasetIndex === 'number') {
    legend.chart.setDatasetVisibility(item.datasetIndex, visible)
    return
  }
  if (typeof item.index === 'number' && legend.chart.getDataVisibility(item.index) !== visible) {
    legend.chart.toggleDataVisibility(item.index)
  }
}

/**
 * First click focuses one series. Further clicks accumulate or remove series.
 * Clicking the sole focused series again, or selecting every item, restores all.
 */
export function focusOrAccumulateLegendClick(
  _event: ChartEvent,
  clickedItem: LegendItem,
  legend: LegendElement<ChartType>
): void {
  const items = legend.legendItems || []
  if (!items.length) return

  const visibleItems = items.filter((item) => itemVisible(legend, item))
  if (visibleItems.length === items.length) {
    for (const item of items) setItemVisible(legend, item, sameItem(item, clickedItem))
  } else if (visibleItems.length === 1 && itemVisible(legend, clickedItem)) {
    for (const item of items) setItemVisible(legend, item, true)
  } else if (!itemVisible(legend, clickedItem)) {
    setItemVisible(legend, clickedItem, true)
  } else if (visibleItems.length > 1) {
    setItemVisible(legend, clickedItem, false)
  }

  legend.chart.update()
}

// Install before any chart component registers the Legend plugin.
if (Legend.defaults) {
  Legend.defaults.onClick = focusOrAccumulateLegendClick
}

// Also cover charts registered before this module in tests or alternate entry points.
if (Chart.defaults.plugins.legend) {
  Chart.defaults.plugins.legend.onClick = focusOrAccumulateLegendClick
}
