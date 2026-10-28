import { InputField } from '../InputField'
import { PasswordField } from '../PasswordField'
import type { LoginMethod } from './LoginMethodSwitch'

type Props = {
  method: LoginMethod
  identifier: string
  password: string
  pending: boolean
  identifierError?: string
  passwordError?: string
  onIdentifierChange: (value: string) => void
  onIdentifierBlur: () => void
  onPasswordChange: (value: string) => void
  onPasswordBlur: () => void
}

export function LoginCredentials(props: Props) {
  const phone = props.method === 'phone'
  return <>
    <InputField id={phone ? 'phone' : 'identifier'} label={phone ? 'Mobile number' : 'Email or NIC number'} icon={phone ? 'phone' : 'mail'}
      type={phone ? 'tel' : 'text'} inputMode={phone ? 'tel' : 'text'} autoComplete={phone ? 'tel' : 'username'} autoCapitalize="none" spellCheck={false} required
      placeholder={phone ? '+94 77 123 4567' : 'Email or NIC number'} value={props.identifier} disabled={props.pending}
      error={props.identifierError} onBlur={props.onIdentifierBlur} onChange={event => props.onIdentifierChange(event.target.value)} />
    <PasswordField key={props.method} id="password" autoComplete="current-password" required value={props.password} disabled={props.pending}
      error={props.passwordError} onBlur={props.onPasswordBlur} onChange={event => props.onPasswordChange(event.target.value)} />
  </>
}
