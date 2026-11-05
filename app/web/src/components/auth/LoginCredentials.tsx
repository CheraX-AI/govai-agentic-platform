import { InputField } from '../InputField'
import { PasswordField } from '../PasswordField'

type Props = {
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
  return <>
    <InputField id="identifier" label="Email or NIC number" icon="mail"
      type="text" inputMode="text" autoComplete="username" autoCapitalize="none" spellCheck={false} required
      placeholder="Email or NIC number" value={props.identifier} disabled={props.pending}
      error={props.identifierError} onBlur={props.onIdentifierBlur} onChange={event => props.onIdentifierChange(event.target.value)} />
    <PasswordField id="password" autoComplete="current-password" required value={props.password} disabled={props.pending}
      error={props.passwordError} onBlur={props.onPasswordBlur} onChange={event => props.onPasswordChange(event.target.value)} />
  </>
}
