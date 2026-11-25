import { useCallback, useEffect, useRef, useState } from 'react'
import {
  getProfile,
  getProfilePhoto,
  saveProfile,
  uploadProfilePhoto,
  ProfileError,
} from '../services/profileApi'
import type { CitizenProfile, ProfileFields } from '../services/profileApi'
import { validateProfile, validateProfilePhoto } from '../validation/profileValidation'

function editable(profile: CitizenProfile): ProfileFields {
  return {
    name_si: profile.name_si,
    name_ta: profile.name_ta,
    date_of_birth: profile.date_of_birth,
    gender: profile.gender,
    address: { ...profile.address },
  }
}
// Owns in-memory profile state, requests and draft reconciliation.
// The page supplies the existing session-rejection action.
export function useProfile(onSessionRejected: () => void) {
  const [today] = useState(() => new Date().toISOString().slice(0, 10))
  const [profile, setProfile] = useState<CitizenProfile | null>(null)
  const [draft, setDraft] = useState<ProfileFields | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [needsReload, setNeedsReload] = useState(false)
  const [photo, setPhoto] = useState<Blob | null>(null)
  const [selection, setSelection] = useState<File | null>(null)
  const [photoError, setPhotoError] = useState('')
  const [photoVersion, setPhotoVersion] = useState(0)
  const [objectUrl, setImageUrl] = useState('')
  const imageUrl = selection || photo ? objectUrl : ''
  const [loadVersion, setLoadVersion] = useState(0)
  const controller = useRef<AbortController | null>(null)
  const gate = useRef(false)
  const latestError = useCallback(
    (cause: unknown) => {
      if (cause instanceof ProfileError && cause.status === 401) {
        onSessionRejected()
        return
      }
      setError(
        cause instanceof ProfileError
          ? cause.message
          : 'Your profile could not be loaded. Please try again.'
      )
    },
    [onSessionRejected]
  )
  useEffect(() => {
    const request = new AbortController()
    controller.current = request
    void getProfile(request.signal)
      .then((value) => {
        if (!request.signal.aborted) {
          setProfile(value)
          setDraft(editable(value))
          setError('')
        }
      })
      .catch((cause) => {
        if (!request.signal.aborted) latestError(cause)
      })
    return () => {
      request.abort()
      controller.current = null
    }
    // A new owner receives a new mounted page through ProtectedRoute.
  }, [loadVersion, latestError])
  useEffect(() => {
    const request = new AbortController()
    if (profile?.has_photo && profile.photo_url) {
      void getProfilePhoto(request.signal)
        .then((value) => {
          if (!request.signal.aborted) {
            setPhoto(value)
            setPhotoError('')
          }
        })
        .catch((cause) => {
          if (!request.signal.aborted) {
            if (cause instanceof ProfileError && cause.status === 401) onSessionRejected()
            else setPhotoError('Your saved photo could not be displayed. You can retry loading it.')
          }
        })
    }
    return () => request.abort()
  }, [profile?.has_photo, profile?.photo_url, profile?.revision, photoVersion, onSessionRejected])
  useEffect(() => {
    const blob = selection ?? photo
    if (!blob) return
    const url = URL.createObjectURL(blob)
    // This effect owns an external browser URL and revokes it on replacement.
    // oxlint-disable-next-line react/set-state-in-effect
    setImageUrl(url)
    return () => URL.revokeObjectURL(url)
  }, [selection, photo])
  const dirty = Boolean(
    profile && draft && JSON.stringify(editable(profile)) !== JSON.stringify(draft)
  )
  useEffect(() => {
    if (!dirty && !selection) return
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    addEventListener('beforeunload', warn)
    return () => removeEventListener('beforeunload', warn)
  }, [dirty, selection])
  function change(field: 'name_si' | 'name_ta' | 'date_of_birth' | 'gender', value: string) {
    setDraft((previous) =>
      previous ? ({ ...previous, [field]: value } as ProfileFields) : previous
    )
    setNotice('')
  }
  function address(field: keyof ProfileFields['address'], value: string) {
    setDraft((previous) =>
      previous ? { ...previous, address: { ...previous.address, [field]: value } } : previous
    )
    setNotice('')
  }
  async function mutate(action: () => Promise<void>) {
    if (gate.current) return
    gate.current = true
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await action()
    } catch (cause) {
      if (!controller.current?.signal.aborted && controller.current) {
        latestError(cause)
        if (!(cause instanceof ProfileError) || cause.status === 409 || cause.status >= 500)
          setNeedsReload(true)
      }
    } finally {
      gate.current = false
      if (controller.current) setBusy(false)
    }
  }
  async function save() {
    if (!draft || !profile || needsReload) return
    const validationError = validateProfile(draft)
    if (validationError) {
      setError(validationError)
      return
    }
    await mutate(async () => {
      const saved = await saveProfile(draft, profile.revision, controller.current?.signal)
      if (!controller.current || controller.current.signal.aborted) return
      setProfile(saved)
      setDraft(editable(saved))
      setNotice('Your personal information has been saved.')
    })
  }
  async function reload(keepEdits: boolean) {
    await mutate(async () => {
      const latest = await getProfile(controller.current?.signal)
      if (!controller.current || controller.current.signal.aborted) return
      setProfile(latest)
      if (!keepEdits) {
        setDraft(editable(latest))
        setSelection(null)
      }
      setNeedsReload(false)
      setNotice(
        keepEdits
          ? 'Latest profile loaded. Your unsaved edits are kept. Review them before saving.'
          : 'Latest saved profile loaded.'
      )
    })
  }

  function choosePhoto(file: File) {
    setPhotoError('')
    setNotice('')
    const validationError = validateProfilePhoto(file)
    if (validationError) {
      setPhotoError(validationError)
      return
    }
    setSelection(file)
  }
  async function uploadPhoto() {
    if (!selection || !profile || needsReload) return
    await mutate(async () => {
      const result = await uploadProfilePhoto(
        selection,
        profile.revision,
        controller.current?.signal
      )
      if (!controller.current || controller.current.signal.aborted) return
      setProfile((previous) => (previous ? { ...previous, ...result } : previous))
      setSelection(null)
      setPhotoError('')
      setNotice('Your profile photo has been saved.')
    })
  }
  function cancel() {
    if (!profile) return
    setDraft(editable(profile))
    setSelection(null)
    setNotice('Unsaved changes discarded.')
    setError('')
  }
  function retryLoad() {
    setError('')
    setLoadVersion((value) => value + 1)
  }
  function retryPhoto() {
    setPhotoVersion((value) => value + 1)
  }
  function cancelPhoto() {
    setSelection(null)
  }
  const initials =
    profile?.full_name
      .trim()
      .split(/\s+/u)
      .slice(0, 2)
      .map((part) => [...part][0])
      .join('')
      .toUpperCase() || 'G'
  return {
    profile,
    draft,
    busy,
    error,
    notice,
    needsReload,
    imageUrl,
    selection,
    photoError,
    initials,
    today,
    dirty,
    change,
    address,
    save,
    reload,
    cancel,
    choosePhoto,
    uploadPhoto,
    retryLoad,
    retryPhoto,
    cancelPhoto,
    reportPhotoError: setPhotoError,
  }
}

export type ProfileController = ReturnType<typeof useProfile>
