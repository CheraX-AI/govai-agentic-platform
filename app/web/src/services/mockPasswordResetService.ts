import type { MockOutcome } from './mockLoginService'

// UI simulation only. Never receives or changes a password or account.
export async function submitMockPasswordReset(outcome: MockOutcome): Promise<MockOutcome> {
  await new Promise(resolve => setTimeout(resolve, 800))
  return outcome
}
