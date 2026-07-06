import { useEffect, useState } from 'react'

/** Ticking clock for live durations; updates once per second only while active. */
export function useJobNow(active: boolean): number {
  const [now, setNow] = useState(0)
  useEffect(() => {
    if (!active) {
      return
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setNow(Date.now())
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [active])
  return now
}
