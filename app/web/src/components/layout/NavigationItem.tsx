import { Icon } from '../Icon'

export function NavigationItem({ label, icon, href, active = false, newChat = false, onActivate }: { label: string; icon: string; href?: string; active?: boolean; newChat?: boolean; onActivate?: () => void }) {
  const content = <><Icon collection="shell" name={icon} /><span>{label}</span>{!href && <span className="sr-only"> (unavailable)</span>}</>
  const className = `navigation-item${active ? ' active' : ''}${newChat ? ' new-chat' : ''}`
  return href ? <a className={className} href={href} onClick={onActivate ? event => { if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return; event.preventDefault(); onActivate() } : undefined} aria-current={active ? 'page' : undefined}>{content}</a> : <button className={className} type="button" disabled title={`${label} unavailable in this preview`}>{content}</button>
}
