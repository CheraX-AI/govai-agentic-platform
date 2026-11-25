import type { ProfileController } from '../../profile/useProfile'
import { ProfileField } from './ProfileField'

export function ProfileForm({ model }: { model: ProfileController }) {
  const {
    profile,
    draft,
    busy,
    today,
    dirty,
    selection,
    needsReload,
    change,
    address,
    save,
    cancel,
  } = model
  if (!profile || !draft) return null
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault()
        void save()
      }}
    >
      <fieldset disabled={busy}>
        <legend className="sr-only">Your personal information</legend>
        <div className="profile-fields">
          <ProfileField
            id="profile-full-name"
            label="Full name (English)"
            locked
            hint="Registered name"
          >
            <input
              id="profile-full-name"
              value={profile.full_name}
              readOnly
              aria-describedby="profile-full-name-hint"
            />
          </ProfileField>
          <ProfileField id="profile-nic" label="NIC number" locked>
            <input id="profile-nic" value={profile.nic} readOnly />
          </ProfileField>
          <ProfileField id="profile-name-si" label="Full name (Sinhala)">
            <input
              id="profile-name-si"
              lang="si"
              value={draft.name_si}
              onChange={(event) => change('name_si', event.target.value)}
              autoComplete="off"
            />
          </ProfileField>
          <ProfileField id="profile-name-ta" label="Full name (Tamil)">
            <input
              id="profile-name-ta"
              lang="ta"
              value={draft.name_ta}
              onChange={(event) => change('name_ta', event.target.value)}
              autoComplete="off"
            />
          </ProfileField>
          <ProfileField id="profile-dob" label="Date of birth">
            <input
              id="profile-dob"
              type="date"
              min="0001-01-01"
              max={today}
              value={draft.date_of_birth}
              onChange={(event) => change('date_of_birth', event.target.value)}
              autoComplete="bday"
            />
          </ProfileField>
          <ProfileField id="profile-gender" label="Gender">
            <select
              id="profile-gender"
              value={draft.gender}
              onChange={(event) => change('gender', event.target.value)}
            >
              <option value="">Select gender</option>
              <option value="male">Male</option>
              <option value="female">Female</option>
            </select>
          </ProfileField>
          <ProfileField id="profile-mobile" label="Mobile number" locked>
            <input id="profile-mobile" type="tel" value={profile.mobile_number} readOnly />
          </ProfileField>
          <ProfileField
            id="profile-email"
            label="Email address"
            locked
            hint="Managed through your account settings"
          >
            <input
              id="profile-email"
              value={profile.email ?? ''}
              placeholder="Not available"
              readOnly
              aria-describedby="profile-email-hint"
            />
          </ProfileField>
        </div>
        <div className="profile-address">
          <h3>Address</h3>
          <div className="profile-fields">
            <div className="profile-wide">
              <ProfileField id="profile-address-1" label="Address line 1">
                <input
                  id="profile-address-1"
                  value={draft.address.line1}
                  onChange={(event) => address('line1', event.target.value)}
                  autoComplete="address-line1"
                />
              </ProfileField>
            </div>
            <ProfileField id="profile-address-2" label="Address line 2 (optional)">
              <input
                id="profile-address-2"
                value={draft.address.line2}
                onChange={(event) => address('line2', event.target.value)}
                autoComplete="address-line2"
              />
            </ProfileField>
            <ProfileField id="profile-city" label="City">
              <input
                id="profile-city"
                value={draft.address.city}
                onChange={(event) => address('city', event.target.value)}
                autoComplete="address-level2"
              />
            </ProfileField>
          </div>
        </div>
        <p className="profile-readonly-note">
          Your registered name, NIC and mobile number cannot be changed here.
        </p>
        <div className="profile-form-actions">
          <button type="button" disabled={!dirty && !selection} onClick={cancel}>
            Cancel
          </button>
          <button className="profile-save" type="submit" disabled={!dirty || needsReload}>
            {busy ? 'Saving…' : 'Save changes'}
          </button>
        </div>
      </fieldset>
    </form>
  )
}
