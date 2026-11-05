import { Icon } from '../Icon'

export function NavigationItem({ label, icon, href, active = false, newChat = false }: { label: string; icon: string; href?: string; active?: boolean; newChat?: boolean }) {
  const content = <><Icon collection="shell" name={icon} /><span>{label}</span>{!href && <span className="sr-only"> (unavailable)</span>}</>
  const className = `navigation-item${active ? ' active' : ''}${newChat ? ' new-chat' : ''}`
  return href ? <a className={className} href={href} aria-current={active ? 'page' : undefined}>{content}</a> : <button className={className} type="button" disabled title={`${label} is not available yet`}>{content}</button>
}
