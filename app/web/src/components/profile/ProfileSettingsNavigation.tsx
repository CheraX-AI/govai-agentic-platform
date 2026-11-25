import { ProfileSettingsIcon } from './ProfileSettingsIcon'

export function ProfileSettingsNavigation({
  busy,
  onLogout,
}: {
  busy: boolean
  onLogout: () => void
}) {
  return (
    <nav className="profile-settings-nav" aria-label="Settings">
      <a href="/app/profile" aria-current="page">
        <ProfileSettingsIcon name="profile" />
        <span>
          <strong>Profile</strong>
          <small>Personal information</small>
        </span>
      </a>
      {[
        ['security', 'Security and login', 'Password, KYC and two-step login'],
        ['subscription', 'Subscription', 'Your plan and features'],
        ['payment', 'Payments and billing', 'Payment methods and receipts'],
        ['notifications', 'Notifications', 'Choose what you hear about'],
        ['language', 'Language', 'English, Sinhala or Tamil'],
        ['appearance', 'Appearance', 'Theme and text size'],
      ].map(([icon, title, subtitle]) => (
        <button key={title} type="button" disabled title="Not available yet">
          <ProfileSettingsIcon name={icon} />
          <span>
            <strong>{title}</strong>
            <small>{subtitle}</small>
          </span>
        </button>
      ))}
      <button className="profile-sign-out" type="button" disabled={busy} onClick={onLogout}>
        <span aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
            <path d="M9 4H5a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4M14 7l5 5-5 5M8 12h11" />
          </svg>
        </span>
        <span>
          <strong>Sign out</strong>
          <small>Sign out of GovAI on this device</small>
        </span>
      </button>
    </nav>
  )
}
