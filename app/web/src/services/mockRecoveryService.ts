import type { MockOutcome } from './mockLoginService'

// Deliberately accepts no identifier: no account lookup, delivery or reset link.
export function submitMockRecovery(outcome: MockOutcome): Promise<MockOutcome> {
  return new Promise(resolve => setTimeout(() => resolve(outcome), 800))
}
