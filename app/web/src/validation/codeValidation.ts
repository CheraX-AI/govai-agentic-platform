export function validateCode(code: string) {
  return !code ? 'Enter the six-digit demo code.' : !/^\d{6}$/.test(code) ? 'Enter all six digits.' : undefined
}
