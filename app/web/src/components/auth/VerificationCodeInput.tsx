import { useRef } from 'react'

export function VerificationCodeInput({ value, onChange, disabled, error }: { value: string[]; onChange: (digits: string[]) => void; disabled: boolean; error?: string }) {
  const inputs = useRef<(HTMLInputElement | null)[]>([])
  function insert(index: number, text: string) {
    if (!/^[0-9]+$/.test(text)) return
    const next = [...value]
    const start = text.length >= 6 ? 0 : index
    const digits = text.slice(0, 6 - start)
    for (let offset = 0; offset < digits.length; offset++) next[start + offset] = digits[offset]
    onChange(next)
    inputs.current[Math.min(5, start + digits.length)]?.focus()
  }
  return <fieldset className="verification-code" disabled={disabled} aria-describedby={error ? 'code-hint code-error' : 'code-hint'}>
    <legend className="sr-only">Six-digit demo verification code</legend>
    {value.map((digit, index) => <input key={index} ref={element => { inputs.current[index] = element }} id={`code-${index}`} aria-label={`Digit ${index + 1} of 6`} aria-invalid={error ? true : undefined} aria-describedby={error ? 'code-hint code-error' : 'code-hint'} required type="text" inputMode="numeric" autoComplete={index === 0 ? 'one-time-code' : 'off'} value={digit}
      onFocus={event => event.currentTarget.select()}
      onChange={event => { const text = event.target.value; if (!text) { const next = [...value]; next[index] = ''; onChange(next) } else insert(index, text) }}
      onPaste={event => { event.preventDefault(); insert(index, event.clipboardData.getData('text').trim()) }}
      onKeyDown={event => {
        if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') { event.preventDefault(); inputs.current[Math.max(0, Math.min(5, index + (event.key === 'ArrowLeft' ? -1 : 1)))]?.focus() }
        if (event.key === 'Backspace' && !digit && index > 0) { event.preventDefault(); const next = [...value]; next[index - 1] = ''; onChange(next); inputs.current[index - 1]?.focus() }
      }} />)}
  </fieldset>
}
