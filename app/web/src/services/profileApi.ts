// Citizen profile requests use the existing HttpOnly browser session. No personal
// details or photo blobs are written to browser storage or sent to the chat API.
export type ProfileFields = {
  name_si: string
  name_ta: string
  date_of_birth: string
  gender: '' | 'male' | 'female'
  address: { line1: string; line2: string; city: string }
}
export type CitizenProfile = ProfileFields & {
  full_name: string
  nic: string
  mobile_number: string
  email: string | null
  revision: number
  has_photo: boolean
  photo_url: '/api/v1/web/profile/photo' | null
}
export type PhotoResult = Pick<CitizenProfile, 'revision' | 'has_photo' | 'photo_url'>
export class ProfileError extends Error {
  status: number
  code: string
  constructor(status: number, message: string, code = 'profile_unavailable') {
    super(message)
    this.name = 'ProfileError'
    this.status = status
    this.code = code
  }
}
export const MAX_PHOTO_BYTES = 2_097_152
const photoPath = '/api/v1/web/profile/photo'
const knownCodes = new Set([
  'revision_conflict',
  'invalid_gender',
  'dob_nic_mismatch',
  'photo_too_large',
  'unsupported_photo_type',
])
const record = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value)
const text = (value: unknown, limit = 2000): value is string =>
  typeof value === 'string' &&
  [...value].length <= limit &&
  ![...value].some(
    (character) => character.codePointAt(0)! < 32 || character.codePointAt(0) === 127
  )
const revision = (value: unknown): value is number =>
  Number.isSafeInteger(value) && (value as number) >= 0
const invalid = () =>
  new ProfileError(502, 'The profile response could not be verified. Please reload your profile.')

function parsePhoto(value: unknown): PhotoResult {
  if (
    !record(value) ||
    !revision(value.revision) ||
    typeof value.has_photo !== 'boolean' ||
    (value.photo_url !== null && value.photo_url !== photoPath) ||
    (!value.has_photo && value.photo_url !== null)
  )
    throw invalid()
  return { revision: value.revision, has_photo: value.has_photo, photo_url: value.photo_url }
}
function parseProfile(value: unknown): CitizenProfile {
  if (
    !record(value) ||
    !text(value.full_name) ||
    !text(value.nic) ||
    !text(value.mobile_number) ||
    !(value.email === null || text(value.email)) ||
    !text(value.name_si, 200) ||
    !text(value.name_ta, 200) ||
    !text(value.date_of_birth, 10) ||
    !['', 'male', 'female'].includes(String(value.gender)) ||
    !record(value.address) ||
    !text(value.address.line1, 200) ||
    !text(value.address.line2, 200) ||
    !text(value.address.city, 100)
  )
    throw invalid()
  return {
    full_name: value.full_name,
    nic: value.nic,
    mobile_number: value.mobile_number,
    email: value.email,
    name_si: value.name_si,
    name_ta: value.name_ta,
    date_of_birth: value.date_of_birth,
    gender: value.gender as ProfileFields['gender'],
    address: { line1: value.address.line1, line2: value.address.line2, city: value.address.city },
    ...parsePhoto(value),
  }
}
function apiBase() {
  let base: URL
  try {
    base = new URL(import.meta.env?.VITE_AUTH_API_URL ?? 'https://localhost:5001')
  } catch {
    throw new ProfileError(503, 'Profile configuration is unavailable.')
  }
  if (
    base.protocol !== 'https:' ||
    base.username ||
    base.password ||
    base.search ||
    base.hash ||
    base.pathname !== '/'
  )
    throw new ProfileError(503, 'Profile configuration is unavailable.')
  return base
}
async function failure(response: Response): Promise<never> {
  const result: unknown = await response.json().catch(() => null)
  const code =
    record(result) && typeof result.code === 'string' && knownCodes.has(result.code)
      ? result.code
      : 'profile_unavailable'
  const messages: Record<number, string> = {
    400: 'Check the profile fields and reload before saving again.',
    401: 'Your session ended. Please log in again.',
    403: 'Your profile request was not permitted. Please check your session.',
    404: 'This profile feature or photo is not available.',
    409: 'Your profile changed elsewhere. Load the latest profile and review your edits before saving again.',
    413: 'The upload or profile is too large. Choose a JPEG or PNG photo up to 2 MiB.',
    415: 'Choose a valid JPEG or PNG image.',
    422:
      code === 'dob_nic_mismatch'
        ? 'The date of birth must match your registered NIC.'
        : 'Check your profile details. Gender must be Male or Female and the date of birth must be valid.',
    428: 'Load your latest profile before uploading a photo.',
    429: 'Too many requests. Please wait before trying again.',
  }
  throw new ProfileError(
    response.status,
    messages[response.status] ??
      'The profile service is unavailable. Reload your profile to check whether your last change was saved.',
    code
  )
}
async function request(
  path: 'profile' | 'profile/photo' | 'csrf',
  options: RequestInit,
  signal?: AbortSignal
) {
  const base = apiBase()
  const timeout = AbortSignal.timeout(30_000)
  let response: Response
  try {
    response = await fetch(new URL('/api/v1/web/' + path, base), {
      ...options,
      credentials: 'include',
      cache: 'no-store',
      redirect: 'error',
      signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
      headers: { ...options.headers, 'X-GovAI-Web': '1' },
    })
  } catch {
    if (signal?.aborted) throw new DOMException('Profile request cancelled.', 'AbortError')
    throw new ProfileError(
      503,
      'The profile request could not be confirmed. Reload your profile to check whether your last change was saved.'
    )
  }
  if (!response.ok) return failure(response)
  return response
}
async function csrf(signal?: AbortSignal): Promise<string> {
  const response = await request('csrf', {}, signal)
  const value: unknown = await response.json().catch(() => null)
  if (
    !record(value) ||
    typeof value.csrf_token !== 'string' ||
    !/^[A-Za-z0-9_-]{43}$/.test(value.csrf_token)
  )
    throw new ProfileError(502, 'Your session could not be confirmed. Please log in again.')
  return value.csrf_token
}
export async function getProfile(signal?: AbortSignal): Promise<CitizenProfile> {
  const response = await request('profile', {}, signal)
  return parseProfile(await response.json().catch(() => null))
}
export async function saveProfile(
  fields: ProfileFields,
  currentRevision: number,
  signal?: AbortSignal
): Promise<CitizenProfile> {
  if (!revision(currentRevision) || currentRevision >= Number.MAX_SAFE_INTEGER)
    throw new ProfileError(400, 'Reload your profile before saving.')
  // Explicit allowlist: never forward a form's extra identity or read-only fields.
  const body = JSON.stringify({
    revision: currentRevision,
    name_si: fields.name_si,
    name_ta: fields.name_ta,
    date_of_birth: fields.date_of_birth,
    gender: fields.gender,
    address: {
      line1: fields.address.line1,
      line2: fields.address.line2,
      city: fields.address.city,
    },
  })
  if (new TextEncoder().encode(body).length > 16_384)
    throw new ProfileError(413, 'Your profile details are too long.')
  const token = await csrf(signal)
  const response = await request(
    'profile',
    { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': token }, body },
    signal
  )
  return parseProfile(await response.json().catch(() => null))
}
export async function getProfilePhoto(signal?: AbortSignal): Promise<Blob> {
  // A plain <img src> cannot attach the browser adapter's required header.
  const response = await request('profile/photo', {}, signal)
  const type = response.headers.get('Content-Type')?.split(';')[0].trim().toLowerCase()
  if (type !== 'image/jpeg' && type !== 'image/png') throw invalid()
  const blob = await response.blob()
  if (blob.size === 0 || blob.size > MAX_PHOTO_BYTES) throw invalid()
  return blob
}
export async function uploadProfilePhoto(
  photo: Blob,
  currentRevision: number,
  signal?: AbortSignal
): Promise<PhotoResult> {
  if (!revision(currentRevision) || currentRevision >= Number.MAX_SAFE_INTEGER)
    throw new ProfileError(400, 'Reload your profile before uploading.')
  if (!['image/jpeg', 'image/png'].includes(photo.type) || photo.size === 0)
    throw new ProfileError(415, 'Choose a JPEG or PNG photo.')
  if (photo.size > MAX_PHOTO_BYTES) throw new ProfileError(413, 'Choose a photo up to 2 MiB.')
  const token = await csrf(signal)
  const response = await request(
    'profile/photo',
    {
      method: 'PUT',
      headers: {
        'Content-Type': photo.type,
        'X-CSRF-Token': token,
        'If-Match': `"${currentRevision}"`,
      },
      body: photo,
    },
    signal
  )
  const result = parsePhoto(await response.json().catch(() => null))
  if (!result.has_photo || result.photo_url !== photoPath) throw invalid()
  return result
}
