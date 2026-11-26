import { useMemo } from 'react';
import { PersonalInformationScreen } from './PersonalInformationScreen';
import { ProfileError } from '../services/profileApi';
import type { CitizenProfile, ProfileApi } from '../services/profileApi';

// Explicit development fixture only. No login, network calls or persistence.
export function PersonalInformationPreviewScreen({ onExit }: { onExit: () => void; }) {
  const api = useMemo<ProfileApi>(() => {
    let profile: CitizenProfile = {
      full_name: 'Synthetic Citizen', nic: '200322812345', mobile_number: '+94771234567', email: 'citizen@example.test',
      name_si: '', name_ta: '', date_of_birth: '2003-08-15', gender: 'male',
      address: { line1: '42 Sample Road', line2: '', city: 'Colombo' }, revision: 0, has_photo: false, photo_url: null,
    };
    let photoUri = '';
    const copy = () => ({ ...profile, address: { ...profile.address } });
    return {
      load: async () => copy(),
      save: async (fields, revision) => {
        if (revision !== profile.revision) throw new ProfileError(409, 'Your profile changed. Load the latest profile.');
        profile = { ...profile, ...fields, address: { ...fields.address }, revision: revision + 1 };
        return copy();
      },
      photo: async () => photoUri,
      upload: async (photo, revision) => {
        if (revision !== profile.revision) throw new ProfileError(409, 'Your profile changed. Load the latest profile.');
        photoUri = photo.uri;
        profile = { ...profile, revision: revision + 1, has_photo: true, photo_url: '/api/v1/profile/photo' };
        return { revision: profile.revision, has_photo: true, photo_url: '/api/v1/profile/photo' };
      },
    };
  }, []);
  return <PersonalInformationScreen api={api} onBack={onExit} />;
}
