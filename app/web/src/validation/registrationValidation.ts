import { isValidEmail, isValidNic } from './loginValidation.ts'

export type RegistrationValues = {
  fullName: string
  nic: string
  phone: string
  email: string
  password: string
  confirmation: string
  terms: boolean
}

// Demo policy only: align with the supplied authentication system in GA-54.
// Count Unicode code points, never UTF-16 units; do not trim or truncate passwords.
export function validateDemoPasswords(password: string, confirmation: string) {
  const passwordValid = [...password].length >= 8 && /[0-9]/.test(password) && /[\p{P}\p{S}]/u.test(password)
  return {
    password: !password ? 'Enter a password.' : !passwordValid ? 'Use at least 8 Unicode characters, an ASCII digit (0-9), and a punctuation or symbol character.' : undefined,
    confirmation: !confirmation ? 'Confirm your password.' : confirmation !== password ? 'Passwords must match exactly.' : undefined,
  }
}
export function normalizeRegistrationPhone(value: string): string | undefined {
  const compact = value.replace(/[\s-]/g, '')
  if (/^07\d{8}$/.test(compact)) return `+94${compact.slice(1)}`
  return /^\+[1-9]\d{6,14}$/.test(compact) ? compact : undefined
}
export function validateRegistration(values: RegistrationValues) {
  return {
    fullName: !values.fullName.trim() ? 'Enter your full name.' : undefined,
    nic: !values.nic.trim() ? 'Enter your NIC number.' : !isValidNic(values.nic) ? 'Use 9 digits followed by V/X, or 12 digits.' : undefined,
    phone: !values.phone.trim() ? 'Enter your mobile number.' : !normalizeRegistrationPhone(values.phone) ? 'Use an international number such as +94771234567.' : undefined,
    email: !values.email.trim() ? 'Enter your email address.' : !isValidEmail(values.email) ? 'Enter a valid email address without internal whitespace.' : undefined,
    password: [...values.password].length < 8 || new TextEncoder().encode(values.password).length > 1024 ? 'Use at least 8 characters and at most 1024 UTF-8 bytes.' : undefined,
    confirmation: !values.confirmation ? 'Confirm your password.' : values.password !== values.confirmation ? 'Passwords must match exactly.' : undefined,
    terms: !values.terms ? 'Select the demo-only agreement checkbox.' : undefined,
  }
}
