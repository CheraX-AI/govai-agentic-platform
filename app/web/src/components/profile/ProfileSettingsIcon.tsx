import type { ReactNode } from 'react'

// Consistent outline icons for the approved profile settings navigation.
const shapes: Record<string, ReactNode> = {
  profile: (
    <>
      <circle cx="12" cy="7" r="3.5" />
      <path d="M5 21v-3a7 7 0 0 1 14 0v3" />
    </>
  ),
  security: (
    <>
      <path d="M12 3 4.5 6v5.5c0 4.4 2.8 7.6 7.5 9.5 4.7-1.9 7.5-5.1 7.5-9.5V6Z" />
      <path d="m8.5 12 2.3 2.3 4.7-4.8" />
    </>
  ),
  subscription: (
    <>
      <path d="m3 6 4.5 4L12 4l4.5 6L21 6l-2 12H5Z" />
      <path d="M5 21h14" />
    </>
  ),
  payment: (
    <>
      <rect x="3" y="5" width="18" height="14" rx="2.5" />
      <path d="M3 10h18M7 15h4" />
    </>
  ),
  notifications: (
    <>
      <path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9Z" />
      <path d="M10 21h4M12 2V1" />
    </>
  ),
  language: (
    <>
      <circle cx="12" cy="12" r="9" />
      <ellipse cx="12" cy="12" rx="4" ry="9" />
      <path d="M3 12h18M5 6.5h14M5 17.5h14" />
    </>
  ),
  appearance: (
    <>
      <path d="M21 12a9 9 0 1 0-9 9h1.2a2.1 2.1 0 0 0 1.7-3.3 1.6 1.6 0 0 1 1.3-2.6H18a3 3 0 0 0 3-3.1Z" />
      <circle cx="7.5" cy="9" r=".8" />
      <circle cx="11" cy="6.5" r=".8" />
      <circle cx="15.5" cy="8" r=".8" />
      <circle cx="6.5" cy="13.5" r=".8" />
    </>
  ),
}

export function ProfileSettingsIcon({ name }: { name: string }) {
  return (
    <svg
      className="settings-menu-icon"
      width="22"
      height="22"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.65"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {shapes[name]}
    </svg>
  )
}
