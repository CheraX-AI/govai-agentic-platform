import { Icon } from '../Icon'

export function NavigationItem({
  label,
  icon,
  href,
  active = false,
  newChat = false,
  onClick,
}: {
  label: string
  icon: string
  href?: string
  active?: boolean
  newChat?: boolean
  onClick?: () => void
}) {
  const content = (
    <>
      <Icon collection="shell" name={icon} />
      <span>{label}</span>
      {!href && !onClick && <span className="sr-only"> (unavailable)</span>}
    </>
  )
  const className = `navigation-item${active ? ' active' : ''}${newChat ? ' new-chat' : ''}`
  if (onClick) {
    return (
      <button
        className={className}
        type="button"
        onClick={onClick}
        aria-current={active ? 'page' : undefined}
      >
        {content}
      </button>
    )
  }
  return href ? (
    <a className={className} href={href} aria-current={active ? 'page' : undefined}>
      {content}
    </a>
  ) : (
    <button className={className} type="button" disabled title={`${label} is not available yet`}>
      {content}
    </button>
  )
}
