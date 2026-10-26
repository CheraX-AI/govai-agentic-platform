export function normalizePhone(value: string): string | undefined {
  const compact = value.replace(/[\s-]/g, '')
  if (/^07\d{8}$/.test(compact)) return `+94${compact.slice(1)}`
  if (/^\+947\d{8}$/.test(compact)) return compact
  return undefined
}

export function validateLogin(phone: string, password: string, humanCheck: boolean) {
  return {
    phone: !phone.trim() ? 'Enter your mobile number.' : !normalizePhone(phone) ? 'Use 07XXXXXXXX or +947XXXXXXXX.' : undefined,
    password: !password ? 'Enter your password.' : undefined,
    humanCheck: !humanCheck ? 'Complete the simulated human check.' : undefined,
  }
}
