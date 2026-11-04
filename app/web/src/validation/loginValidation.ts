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

// Provisional demo formats only; not verification of identity or email ownership.
export function isValidNic(value: string) {
  return /^(?:\d{9}[vx]|\d{12})$/i.test(value.trim())
}

export function isValidEmail(value: string) {
  return /^[^\s@]+@[^\s@.]+(?:\.[^\s@.]+)+$/.test(value.trim())
}

export function validateNicEmailLogin(identifier: string, password: string, humanCheck: boolean) {
  const value = identifier.trim()
  const nic = isValidNic(value)
  const email = isValidEmail(value)
  return {
    identifier: !value ? 'Enter your email or NIC number.' : !nic && !email ? 'Enter a valid email, or a NIC with 9 digits and V/X or 12 digits.' : undefined,
    password: !password ? 'Enter your password.' : undefined,
    humanCheck: !humanCheck ? 'Complete the simulated human check.' : undefined,
  }
}
