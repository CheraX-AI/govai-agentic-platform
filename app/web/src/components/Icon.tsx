export function Icon({ name, collection = 'login' }: { name: string; collection?: 'login' | 'shell' }) {
  return <img className={collection === 'shell' ? 'shell-icon' : 'icon'} src={`/assets/figma/${collection}/${name}.svg`} alt="" aria-hidden="true" />
}
