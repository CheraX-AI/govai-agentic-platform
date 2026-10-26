import type { InputHTMLAttributes, ReactNode } from 'react'

export function Checkbox({ children, ...props }: InputHTMLAttributes<HTMLInputElement> & { children: ReactNode }) {
  return <label className="checkbox-label"><input {...props} type="checkbox" /><span>{children}</span></label>
}
