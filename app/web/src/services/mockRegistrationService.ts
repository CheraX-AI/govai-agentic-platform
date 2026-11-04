import type { MockOutcome } from './mockLoginService'

// Local UI simulation; accepts no registration data and creates no account.
export async function submitMockRegistration(outcome: MockOutcome): Promise<MockOutcome> {
  await new Promise<void>(resolve => window.setTimeout(resolve, 800))
  return outcome
}
