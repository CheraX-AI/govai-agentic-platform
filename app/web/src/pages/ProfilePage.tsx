import { useAuth } from '../auth/AuthContext'
import { useProfile } from '../profile/useProfile'
import { ProfileSettingsNavigation } from '../components/profile/ProfileSettingsNavigation'
import { ProfilePhoto } from '../components/profile/ProfilePhoto'
import { ProfileForm } from '../components/profile/ProfileForm'
import '../styles/profile.css'

export default function ProfilePage({ onLogout }: { onLogout: () => void }) {
  const { retry } = useAuth()
  const model = useProfile(retry)
  const { profile, draft, busy, error, notice, needsReload, retryLoad, reload } = model
  return (
    <section className="profile-settings" aria-labelledby="settings-heading">
      <header className="profile-heading">
        <h1 id="settings-heading">Settings</h1>
        <p>Manage your profile, security and how GovAI works for you.</p>
      </header>
      <div className="profile-settings-grid">
        <ProfileSettingsNavigation busy={busy} onLogout={onLogout} />
        <div className="profile-card" aria-busy={busy || (!profile && !error)}>
          <h2>Personal information</h2>
          <p className="profile-description">
            View your registered information and keep your profile up to date.
          </p>
          {!profile || !draft ? (
            <div className="profile-loading">
              {error ? (
                <>
                  <p role="alert">{error}</p>
                  <button type="button" onClick={retryLoad}>
                    Try again
                  </button>
                </>
              ) : (
                <p role="status">Loading your personal information…</p>
              )}
            </div>
          ) : (
            <>
              {error && (
                <div className="profile-alert" role="alert">
                  {error}
                </div>
              )}
              {notice && (
                <p className="profile-notice" role="status">
                  {notice}
                </p>
              )}
              {needsReload && (
                <div className="profile-reconcile">
                  <p>Your edits are still here. Load the latest saved profile before continuing.</p>
                  <button type="button" disabled={busy} onClick={() => void reload(true)}>
                    Load latest and keep my edits
                  </button>
                  <button type="button" disabled={busy} onClick={() => void reload(false)}>
                    Discard edits and load latest
                  </button>
                </div>
              )}
              <ProfilePhoto model={model} />
              <ProfileForm model={model} />
            </>
          )}
        </div>
      </div>
    </section>
  )
}
