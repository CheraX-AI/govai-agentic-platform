import { useRef } from 'react'
import type { ProfileController } from '../../profile/useProfile'

export function ProfilePhoto({ model }: { model: ProfileController }) {
  const fileInput = useRef<HTMLInputElement>(null)
  const {
    profile,
    busy,
    needsReload,
    imageUrl,
    selection,
    photoError,
    initials,
    choosePhoto,
    uploadPhoto,
    cancelPhoto,
    retryPhoto,
    reportPhotoError,
  } = model
  if (!profile) return null
  return (
    <>
      <div className="profile-photo-row">
        <div className="profile-avatar-wrap">
          <div className="profile-avatar">
            {imageUrl ? (
              <img
                src={imageUrl}
                alt={selection ? 'Selected profile photo preview' : 'Your profile photo'}
                onError={() =>
                  reportPhotoError(
                    'The selected image cannot be displayed. Choose another JPEG or PNG.'
                  )
                }
              />
            ) : (
              <span>{initials}</span>
            )}
          </div>
          <button
            className="profile-camera-button"
            type="button"
            aria-label="Change profile photo"
            disabled={busy || needsReload}
            onClick={() => fileInput.current?.click()}
          >
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              aria-hidden="true"
            >
              <path d="M8 5 10 3h4l2 2h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2Z" />
              <circle cx="12" cy="12" r="4" />
            </svg>
          </button>
        </div>
        <div>
          <input
            ref={fileInput}
            className="sr-only"
            type="file"
            accept="image/jpeg,image/png"
            aria-label="Choose profile photo"
            disabled={busy || needsReload}
            onChange={(event) => {
              const file = event.target.files?.[0]
              event.target.value = ''
              if (file) choosePhoto(file)
            }}
          />
          <button
            className="profile-photo-button"
            type="button"
            disabled={busy || needsReload}
            onClick={() => fileInput.current?.click()}
          >
            Change photo
          </button>
          <small>JPG or PNG, up to 2 MiB</small>
          {selection && (
            <div className="profile-photo-actions">
              <span>Photo selected — not yet saved.</span>
              <button
                type="button"
                disabled={busy || needsReload}
                onClick={() => void uploadPhoto()}
              >
                Upload photo
              </button>
              <button type="button" disabled={busy} onClick={cancelPhoto}>
                Cancel photo
              </button>
            </div>
          )}
        </div>
      </div>
      {photoError && (
        <div className="profile-photo-error" role="alert">
          {photoError}{' '}
          {profile.photo_url && !selection && (
            <button type="button" disabled={busy} onClick={retryPhoto}>
              Retry photo
            </button>
          )}
        </div>
      )}
    </>
  )
}
