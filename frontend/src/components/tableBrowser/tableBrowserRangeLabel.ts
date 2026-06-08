import type { useT } from '../../i18n'

export function buildTableBrowserRangeLabel(
  t: ReturnType<typeof useT>,
  formatInt: (n: number) => string,
  rowCount: number,
  rangeStart: number,
  rangeEnd: number,
  totalRows: number | null,
): string {
  if (rowCount === 0) {
    return t('table_browser.range_empty')
  }
  if (totalRows != null) {
    return t('table_browser.range_of_total', {
      start: formatInt(rangeStart),
      end: formatInt(rangeEnd),
      total: formatInt(totalRows),
    })
  }
  return t('table_browser.range_unknown_total', {
    start: formatInt(rangeStart),
    end: formatInt(rangeEnd),
  })
}
