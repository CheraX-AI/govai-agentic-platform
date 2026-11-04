export type CodeOutcome = 'success' | 'invalid' | 'expired'
export type DemoChallenge = { expiresAt: number; resendAt: number }
export const demoCode = '123456'
export function createDemoChallenge(now = Date.now()): DemoChallenge {
  return { expiresAt: now + 300_000, resendAt: now + 45_000 }
}
export function secondsUntil(deadline: number, now = Date.now()) {
  return Math.max(0, Math.ceil((deadline - now) / 1000))
}
// Local simulation only. No phone, credentials, delivery, tokens or real expiry enforcement.
export async function verifyMockCode(code: string, challenge: DemoChallenge, outcome: CodeOutcome): Promise<CodeOutcome> {
  await new Promise(resolve => setTimeout(resolve, 800))
  if (Date.now() >= challenge.expiresAt || outcome === 'expired') return 'expired'
  return code === demoCode && outcome === 'success' ? 'success' : 'invalid'
}
export async function resendMockCode(challenge: DemoChallenge): Promise<DemoChallenge> {
  if (Date.now() < challenge.resendAt) throw Error('Demo cooldown active')
  await new Promise(resolve => setTimeout(resolve, 800))
  return createDemoChallenge()
}
