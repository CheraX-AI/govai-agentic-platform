import { useState } from 'react'
import { Icon } from './Icon'
import { InputField } from './InputField'
import type { InputFieldProps } from './InputField'

export function PasswordField(props: Omit<InputFieldProps, 'label' | 'icon' | 'type' | 'trailing'>) {
  const [visible, setVisible] = useState(false)
  // A submission disables the field; always conceal it again before a retry.
  if (props.disabled && visible) setVisible(false)
  return <InputField {...props} label="Password" icon="lock" type={visible ? 'text' : 'password'}
    trailing={<button type="button" className="icon-button" disabled={props.disabled}
      aria-label={visible ? 'Hide password' : 'Show password'} aria-pressed={visible}
      onClick={() => setVisible(!visible)}><Icon name="eye" /></button>} />
}
