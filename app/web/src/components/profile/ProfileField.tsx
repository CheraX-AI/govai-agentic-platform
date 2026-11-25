import type { ReactNode } from 'react'
import { Icon } from '../Icon'

export function ProfileField({
  id,
  label,
  hint,
  locked,
  children,
}: {
  id: string
  label: string
  hint?: string
  locked?: boolean
  children: ReactNode
}) {
  return (
    <div className="profile-field">
      <label htmlFor={id}>
        {label}
        {locked && <span className="sr-only"> (read only)</span>}
      </label>
      <div className={`profile-input${locked ? ' is-readonly' : ''}`}>
        <Icon name={locked ? 'lock' : 'register-user'} />
        {children}
      </div>
      {hint && <small id={`${id}-hint`}>{hint}</small>}
    </div>
  )
}
