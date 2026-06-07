/**
 * Converts result rows from the API into a format compatible with Recharts.
 * Assumes the first column is the name/label and the second (optional) is the value.
 *
 * @param rows - 2D array of values from the API
 * @returns Array of objects with name and value properties
 */
export function rowsToChartData(rows: any[][] | undefined) {
  return (
    rows?.map((row) => {
      const obj: { name: string; value?: number } = { name: String(row[0]) }
      if (row[1] !== undefined) {
        obj.value = Number(row[1])
        if (isNaN(obj.value)) {
          obj.value = 0
        }
      }
      return obj
    }) ?? []
  )
}
