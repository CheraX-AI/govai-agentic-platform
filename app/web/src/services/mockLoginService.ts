export type MockOutcome = 'success' | 'error'

// UI simulation only. Credentials are deliberately not passed to this adapter.
export async function submitMockLogin(outcome: MockOutcome): Promise<MockOutcome> {
  await new Promise<void>((resolve) => window.setTimeout(resolve, 800))
  return outcome
}
