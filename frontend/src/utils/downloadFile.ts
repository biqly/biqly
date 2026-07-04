/** Download a text payload as a file via a temporary object URL. */
export function downloadTextFile(
  filename: string,
  text: string,
  mime = 'application/yaml;charset=utf-8;',
): void {
  const blob = new Blob([text], { type: mime })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
