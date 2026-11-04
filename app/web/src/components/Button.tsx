import type { ButtonHTMLAttributes } from 'react'

export function Button({ children, pending, className, type = 'button', ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { pending?: boolean }) {
  return <button {...props} type={type} className={['primary-button', className].filter(Boolean).join(' ')} disabled={props.disabled || pending}>
    {pending && <span className="spinner" aria-hidden="true" />}{children}
  </button>
}
