export function validateTrafficSplits(splits: number[]): boolean {
  const sum = splits.reduce((a, b) => a + b, 0)
  return sum === 100
}

export function canStartExperiment(totalTraffic: number, controlCount: number): boolean {
  return totalTraffic === 100 && controlCount === 1
}

export function canEditVariants(status: string | undefined): boolean {
  return status === 'draft'
}

export function isExperimentActive(status: string | undefined): boolean {
  return status === 'running' || status === 'paused'
}
