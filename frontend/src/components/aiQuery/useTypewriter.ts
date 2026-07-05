import { useEffect, useState } from 'react'

export interface UseTypewriterOptions {
  /** Milliseconds between reveal steps. */
  stepMs?: number
  /** Characters revealed per step. */
  charsPerStep?: number
}

export interface UseTypewriterResult {
  /** The portion of `text` revealed so far. */
  shown: string
  /** True once the full text is visible (or immediately when animation is off). */
  done: boolean
}

const DEFAULT_STEP_MS = 18
const DEFAULT_CHARS_PER_STEP = 2

/** SSR-safe check for the OS "reduce motion" preference. */
export function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false
  }
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * Initial revealed text. When motion is reduced (or the text is empty) the
 * full text is shown instantly; otherwise animation starts from nothing.
 */
export function initialShown(text: string, reduced: boolean): string {
  return reduced || text === '' ? text : ''
}

/** Reveal `charsPerStep` more characters of `text`, clamped to its length. */
export function advanceShown(shown: string, text: string, charsPerStep: number): string {
  const next = Math.min(shown.length + Math.max(1, charsPerStep), text.length)
  return text.slice(0, next)
}

/**
 * Progressively reveals `text` like a typewriter. Respects
 * `prefers-reduced-motion` (shows the full text immediately), restarts cleanly
 * when `text` changes, and clears its timer on unmount.
 */
export function useTypewriter(text: string, opts: UseTypewriterOptions = {}): UseTypewriterResult {
  const stepMs = opts.stepMs ?? DEFAULT_STEP_MS
  const charsPerStep = opts.charsPerStep ?? DEFAULT_CHARS_PER_STEP
  const reduced = prefersReducedMotion()

  const [shown, setShown] = useState(() => initialShown(text, reduced))
  const [prevKey, setPrevKey] = useState({ text, reduced })

  // Reset synchronously during render when the text (or motion preference)
  // changes, so the animation restarts cleanly without a setState-in-effect
  // cascade. Mirrors the repo's useResetStateOnDepsChange pattern.
  if (prevKey.text !== text || prevKey.reduced !== reduced) {
    setPrevKey({ text, reduced })
    setShown(initialShown(text, reduced))
  }

  useEffect(() => {
    // Nothing to animate: reduced motion / empty text already show full text.
    if (reduced || text === '') {
      return
    }
    const id = setInterval(() => {
      setShown((prev) => {
        const next = advanceShown(prev, text, charsPerStep)
        if (next.length >= text.length) {
          clearInterval(id)
        }
        return next
      })
    }, stepMs)
    return () => clearInterval(id)
  }, [text, reduced, stepMs, charsPerStep])

  return { shown, done: shown === text }
}
