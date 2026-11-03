import type { InputHTMLAttributes, ReactNode } from 'react'
import { Icon } from './Icon'

export type InputFieldProps = InputHTMLAttributes<HTMLInputElement> & {
  id: string
  label: string
  icon: string
  error?: string
  trailing?: ReactNode
}

export function InputField({ id, label, icon, error, trailing, ...props }: InputFieldProps) {
  const describedBy = [props['aria-describedby'], error ? `${id}-error` : undefined].filter(Boolean).join(' ') || undefined
  return <div className="input-group">
    <label htmlFor={id}>{label}</label>
    <div className={`input-shell${error ? ' invalid' : ''}`}>
      <Icon name={icon} />
      <input {...props} id={id} aria-invalid={error ? true : props['aria-invalid']} aria-describedby={describedBy} />
      {trailing}
    </div>
    {error && <p id={`${id}-error`} className="field-error">{error}</p>}
  </div>
}
