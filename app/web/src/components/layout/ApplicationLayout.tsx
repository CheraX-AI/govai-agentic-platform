import { useEffect, useRef } from 'react'
import type { KeyboardEvent, ReactNode } from 'react'
import { Sidebar } from './Sidebar'
import type { ApplicationPage } from './Sidebar'
import { ContentContainer } from './ContentContainer'
import { TopBar } from './TopBar'
import '../../styles/application.css'

export function ApplicationLayout({
  page,
  children,
  onLogout,
  onNewChat,
  conversationAvailable,
}: {
  page: ApplicationPage
  children: ReactNode
  onLogout: () => void
  onNewChat?: () => void
  conversationAvailable?: boolean
}) {
  const dialog = useRef<HTMLDialogElement>(null)
  const opener = useRef<HTMLElement | null>(null)

  function closeMenu() {
    dialog.current?.close()
    opener.current?.focus()
  }

  function containMenuFocus(event: KeyboardEvent<HTMLDialogElement>) {
    if (event.key !== 'Tab') return
    const controls = [
      ...event.currentTarget.querySelectorAll<HTMLElement>('a[href], button:not(:disabled)'),
    ].filter(control => control.getClientRects().length > 0)
    const first = controls[0]
    const last = controls[controls.length - 1]

    // Some browsers tab to the scrollable dialog itself. Keep both endpoints usable.

    if (
      event.shiftKey &&
      (document.activeElement === first || document.activeElement === event.currentTarget)
    ) {
      event.preventDefault()
      last?.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first?.focus()
    }
  }

  useEffect(() => {
    const media = matchMedia('(min-width: 901px)')
    function onResize() {

      // The menu opener disappears on desktop; focus visible content instead.

      if (media.matches && dialog.current?.open) {
        dialog.current.close()
        document.getElementById('application-content')?.focus()
      }
    }
    media.addEventListener('change', onResize)
    return () => media.removeEventListener('change', onResize)
  }, [])

  return (
    <div className="application-layout">
      <a
        className="skip-link"
        href="#application-content"
        onClick={event => {
          event.preventDefault()
          document.getElementById('application-content')?.focus()
        }}
      >
        Skip to content
      </a>
      <div className="application-body">
        <aside className="desktop-sidebar">
          <Sidebar
            page={page}
            onLogout={onLogout}
            onNewChat={onNewChat}
            conversationAvailable={conversationAvailable}
          />
        </aside>
        <div className="application-main">
          <TopBar onOpenMenu={button => {
            opener.current = button
            dialog.current?.showModal()
          }} />
          <ContentContainer page={page}>{children}</ContentContainer>
        </div>
      </div>
      <dialog
        ref={dialog}
        id="mobile-navigation"
        className="navigation-dialog"
        aria-labelledby="navigation-title"
        onKeyDown={containMenuFocus}
        onCancel={event => {
          event.preventDefault()
          closeMenu()
        }}
        onClick={event => {
          if (event.target === event.currentTarget) closeMenu()
        }}
      >
        <div className="navigation-drawer">
          <div className="drawer-heading">
            <h2 id="navigation-title">Navigation</h2>
            <button type="button" autoFocus onClick={closeMenu}>Close menu</button>
          </div>
          <div className="navigation-content" onClick={event => {

            // Navigation commands such as New chat are buttons, not just links.

            // Disclosures stay open; notification navigation closes the drawer.
            const target = event.target as HTMLElement
            const disclosure = target.closest('.sidebar-account-settings, .notification-disclosure')
            const navigate = target.closest('[data-close-navigation]')
            if (target.closest('a[href], button:not(:disabled)') && (!disclosure || navigate)) {
              closeMenu()
            }
          }}>
            <Sidebar
              page={page}
              onLogout={onLogout}
              onNewChat={onNewChat}
              conversationAvailable={conversationAvailable}
            />
          </div>
        </div>
      </dialog>
    </div>
  )
}
