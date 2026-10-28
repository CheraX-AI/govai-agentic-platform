import { Icon } from '../Icon'

export type LoginMethod = 'phone' | 'nic-email'

export function LoginMethodSwitch({ method, disabled, onChange }: { method: LoginMethod; disabled: boolean; onChange: (method: LoginMethod) => void }) {
  return <div className="login-method" role="group" aria-label="Login method">
    <button type="button" className={method === 'phone' ? 'selected-method' : 'available-method'} aria-pressed={method === 'phone'} disabled={disabled} onClick={() => onChange('phone')}>
      <Icon name={method === 'phone' ? 'mobile-tab' : 'mobile-tab-inactive'} />Mobile number
    </button>
    <button type="button" className={method === 'nic-email' ? 'selected-method' : 'available-method'} aria-pressed={method === 'nic-email'} disabled={disabled} onClick={() => onChange('nic-email')}>
      <Icon name={method === 'nic-email' ? 'nic-email-tab-active' : 'nic-email-tab'} />NIC or email
    </button>
  </div>
}
