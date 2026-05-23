const HTML_TAG_RE = /<[^>]*>/g
const WHITESPACE_RE = /\s+/g

export function plainTextFromHTML(text: string): string {
  return text.replace(HTML_TAG_RE, ' ').replace(WHITESPACE_RE, ' ').trim()
}
