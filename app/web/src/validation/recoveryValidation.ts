import { isValidNic } from './loginValidation.ts'

export function validateRecovery(nic: string) {
  return {
    nic: !nic.trim() ? 'Enter your NIC number.' : !isValidNic(nic) ? 'Use 9 digits followed by V/X, or 12 digits.' : undefined,
  }
}
