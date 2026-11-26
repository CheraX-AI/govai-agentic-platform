import { useEffect, useRef, useState } from 'react';
import { editableProfile, ProfileError, validateProfile } from '../services/profileApi';
import type { CitizenProfile, ProfileApi, ProfileFields, ProfilePhoto } from '../services/profileApi';
import { pickProfilePhoto } from '../services/profilePhotoPicker';

// Authenticated profile state stays in memory and is discarded on unmount.
export function useCitizenProfile(api: ProfileApi) {
  const [profile, setProfile] = useState<CitizenProfile | null>(null);
  const [draft, setDraft] = useState<ProfileFields | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [needsReload, setNeedsReload] = useState(false);
  const [selectedPhoto, setSelectedPhoto] = useState<ProfilePhoto | null>(null);
  const [savedPhoto, setSavedPhoto] = useState('');
  const [photoError, setPhotoError] = useState('');
  const [loadVisit, setLoadVisit] = useState(0);
  const [photoVisit, setPhotoVisit] = useState(0);
  const active = useRef<AbortController | null>(null);
  const gate = useRef(false);

  useEffect(() => {
    const controller = new AbortController();
    active.current = controller;
    void api.load(controller.signal).then(value => {
      if (!controller.signal.aborted) { setProfile(value); setDraft(editableProfile(value)); setError(''); }
    }).catch(cause => {
      if (!controller.signal.aborted) setError(cause instanceof ProfileError ? cause.message : 'Unable to load your profile. Check your session and try again.');
    });
    return () => { controller.abort(); active.current = null; };
  }, [api, loadVisit]);
  useEffect(() => {
    const controller = new AbortController();
    if (profile?.has_photo && profile.photo_url) {
      void api.photo(controller.signal).then(uri => {
        if (!controller.signal.aborted) { setSavedPhoto(uri); setPhotoError(''); }
      }).catch(() => {
        if (!controller.signal.aborted) setPhotoError('Your saved photo could not be displayed. Try loading it again.');
      });
    }
    return () => controller.abort();
  }, [api, profile?.has_photo, profile?.photo_url, profile?.revision, photoVisit]);

  function change<K extends keyof Omit<ProfileFields, 'address'>>(field: K, value: ProfileFields[K]) {
    setDraft(previous => previous ? { ...previous, [field]: value } : previous);
    setNotice('');
  }
  function changeAddress(field: keyof ProfileFields['address'], value: string) {
    setDraft(previous => previous ? { ...previous, address: { ...previous.address, [field]: value } } : previous);
    setNotice('');
  }
  async function mutate(action: () => Promise<void>) {
    if (gate.current || !active.current) return;
    gate.current = true; setBusy(true); setError(''); setNotice('');
    try { await action(); }
    catch (cause) {
      if (active.current && !active.current.signal.aborted) {
        setError(cause instanceof ProfileError ? cause.message : 'The request could not be confirmed. Reload your profile before trying again.');
        if (!(cause instanceof ProfileError) || cause.status === 409 || cause.status >= 500) setNeedsReload(true);
      }
    } finally { gate.current = false; if (active.current) setBusy(false); }
  }
  async function save() {
    if (!profile || !draft || needsReload) return;
    const invalid = validateProfile(draft);
    if (invalid) { setError(invalid); return; }
    await mutate(async () => {
      const result = await api.save(draft, profile.revision, active.current?.signal);
      if (!active.current || active.current.signal.aborted) return;
      setProfile(result); setDraft(editableProfile(result)); setNotice('Your personal information has been saved.');
    });
  }
  async function reload(keepEdits: boolean) {
    await mutate(async () => {
      const latest = await api.load(active.current?.signal);
      if (!active.current || active.current.signal.aborted) return;
      setProfile(latest);
      if (!keepEdits) { setDraft(editableProfile(latest)); setSelectedPhoto(null); }
      setNeedsReload(false);
      setNotice(keepEdits ? 'Latest profile loaded. Your edits are kept. Review them before saving.' : 'Latest saved profile loaded.');
    });
  }
  async function choosePhoto() {
    if (gate.current || needsReload) return;
    gate.current = true; setBusy(true); setPhotoError(''); setNotice('');
    try {
      const photo = await pickProfilePhoto();
      if (active.current && !active.current.signal.aborted && photo) setSelectedPhoto(photo);
    } catch (cause) {
      if (active.current) setPhotoError(cause instanceof ProfileError ? cause.message : 'Photo selection could not be completed. Please try again.');
    } finally { gate.current = false; if (active.current) setBusy(false); }
  }
  async function uploadPhoto() {
    if (!profile || !selectedPhoto || needsReload) return;
    await mutate(async () => {
      const result = await api.upload(selectedPhoto, profile.revision, active.current?.signal);
      if (!active.current || active.current.signal.aborted) return;
      setProfile(previous => previous ? { ...previous, ...result } : previous);
      setSelectedPhoto(null); setPhotoError(''); setNotice('Your profile photo has been saved.');
    });
  }
  function cancel() {
    if (profile) setDraft(editableProfile(profile));
    setSelectedPhoto(null); setError(''); setNotice('Unsaved changes discarded.');
  }
  function retryLoad() { setError(''); setLoadVisit(value => value + 1); }
  function retryPhoto() { setPhotoVisit(value => value + 1); }
  function cancelPhoto() { setSelectedPhoto(null); }
  const dirty = Boolean(profile && draft && JSON.stringify(editableProfile(profile)) !== JSON.stringify(draft));
  const initials = profile?.full_name.trim().split(/\s+/u).slice(0, 2).map(part => [...part][0]).join('').toUpperCase() || 'G';
  return {
    profile, draft, error, notice, busy, needsReload, selectedPhoto, photoError, initials, dirty,
    imageUri: selectedPhoto?.uri ?? (profile?.has_photo && profile.photo_url ? savedPhoto : ''),
    change, changeAddress, save, reload, choosePhoto, uploadPhoto, cancel, cancelPhoto, retryLoad, retryPhoto,
    reportPhotoError: () => setPhotoError('The photo could not be displayed. Choose another JPEG or PNG.')
  };
}
export type CitizenProfileController = ReturnType<typeof useCitizenProfile>;
