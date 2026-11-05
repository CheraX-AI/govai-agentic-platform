// Format checks only; the backend and Keycloak verify identity and email ownership.
export function isValidNic(value: string) {
  return /^(?:\d{9}[vx]|\d{12})$/i.test(value.trim())
}

export function isValidEmail(value: string) {
  return /^[^\s@]+@[^\s@.]+(?:\.[^\s@.]+)+$/.test(value.trim())
}

export function validateNicEmailLogin(identifier: string, password: string) {
  const value = identifier.trim()
  const nic = isValidNic(value)
  const email = isValidEmail(value)
  return {
    identifier: !value ? 'Enter your email or NIC number.' : !nic && !email ? 'Enter a valid email, or a NIC with 9 digits and V/X or 12 digits.' : undefined,
    password: !password ? 'Enter your password.' : undefined,
  }
}
