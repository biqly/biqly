/** Safe error-to-string: returns `Error.message` for Error instances, otherwise `String(value)`. */
export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
