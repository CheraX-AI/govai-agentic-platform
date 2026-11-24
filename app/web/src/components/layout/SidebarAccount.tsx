import { useEffect, useRef, useState } from 'react'
import { Icon } from '../Icon'

// Profile row at the bottom of the sidebar; the gear opens the account menu.
export function SidebarAccount({ onLogout }: { onLogout: () => void }) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return
    root.current?.querySelector<HTMLElement>('.sidebar-account-menu button')?.focus()
    function dismiss(event: PointerEvent) {
      if (!root.current?.contains(event.target as Node)) setOpen(false)
    }
    function escape(event: globalThis.KeyboardEvent) {
      if (event.key === 'Escape') { event.preventDefault(); setOpen(false); trigger.current?.focus() }
    }
    document.addEventListener('pointerdown', dismiss)
    document.addEventListener('keydown', escape)
    return () => { document.removeEventListener('pointerdown', dismiss); document.removeEventListener('keydown', escape) }
  }, [open])

  return (
    <div
      className="sidebar-account"
      ref={root}
      onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setOpen(false) }}
    >
      {open && (
        <div id="sidebar-account-menu" className="sidebar-account-menu" role="menu" aria-label="Account">
          <button type="button" role="menuitem" onClick={() => { setOpen(false); onLogout() }}>
            Log out
          </button>
        </div>
      )}
      <span className="user-avatar" aria-hidden="true">G</span>
      <strong>GovAI user</strong>
      <button type="button" className="sidebar-account-icon" disabled aria-label="Notifications unavailable" title="Notifications are not available yet">
        <Icon collection="shell" name="notifications" />
      </button>
      <span className="sidebar-account-divider" aria-hidden="true" />
      <button
        ref={trigger}
        type="button"
        className="sidebar-account-icon sidebar-account-settings"
        aria-label="Account settings"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls="sidebar-account-menu"
        onClick={() => setOpen(previous => !previous)}
      >
        <Icon collection="shell" name="settings" />
      </button>
    </div>
  )
}
