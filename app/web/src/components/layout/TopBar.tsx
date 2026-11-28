// Keep the navigation opener on small screens; theme controls are deferred.

export function TopBar({ onOpenMenu }: { onOpenMenu: (opener: HTMLElement) => void }) {
  return (
    <header className="top-bar-slim">
      <button
        className="menu-toggle"
        type="button"
        aria-haspopup="dialog"
        aria-controls="mobile-navigation"
        onClick={event => onOpenMenu(event.currentTarget)}
      >Menu</button>
    </header>
  )
}
