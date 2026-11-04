import { isValidEmail, isValidNic, normalizePhone } from './loginValidation.ts'

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
export function validateRegistration(values: RegistrationValues) {
  return {
    fullName: !values.fullName.trim() ? 'Enter your full name.' : undefined,
    nic: !values.nic.trim() ? 'Enter your NIC number.' : !isValidNic(values.nic) ? 'Use 9 digits followed by V/X, or 12 digits.' : undefined,
    phone: !values.phone.trim() ? 'Enter your mobile number.' : !normalizePhone(values.phone) ? 'Use 07XXXXXXXX or +947XXXXXXXX.' : undefined,
    email: !values.email.trim() ? 'Enter your email address.' : !isValidEmail(values.email) ? 'Enter a valid email address without internal whitespace.' : undefined,
    ...validateDemoPasswords(values.password, values.confirmation),
    terms: !values.terms ? 'Select the demo-only agreement checkbox.' : undefined,
  }
}
