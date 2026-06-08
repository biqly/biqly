export function pickValidIdOrFirst(selectedId: string, items: readonly { id: string }[]): string {
  if (selectedId && items.some((item) => item.id === selectedId)) {
    return selectedId
  }
  return items[0]?.id ?? ''
}

export function pickValidId(selectedId: string, items: readonly { id: string }[]): string {
  if (selectedId && items.some((item) => item.id === selectedId)) {
    return selectedId
  }
  return ''
}

export function pickPublishedModelId(
  selectedId: string,
  models: readonly { id: string; status?: string }[],
): string {
  if (selectedId && models.some((model) => model.id === selectedId)) {
    return selectedId
  }
  const published = models.find((model) => model.status === 'published')
  return published?.id ?? models[0]?.id ?? ''
}
