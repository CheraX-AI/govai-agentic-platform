import { validateApiBase } from './apiConfig';
import { boundedJson, utf8Length } from './publicRequest';

export type ProfileFields = {
  name_si: string;
  name_ta: string;
  date_of_birth: string;
  gender: '' | 'male' | 'female';
  address: { line1: string; line2: string; city: string; };
};
export type CitizenProfile = ProfileFields & {
  full_name: string;
  nic: string;
  mobile_number: string;
  email: string | null;
  revision: number;
  has_photo: boolean;
  photo_url: '/api/v1/profile/photo' | null;
};
export type ProfilePhoto = { uri: string; data: ArrayBuffer; contentType: 'image/jpeg' | 'image/png'; };
export type PhotoResult = Pick<CitizenProfile, 'revision' | 'has_photo' | 'photo_url'>;
export type ProfileAccess = <T>(action: (token: string) => Promise<T>) => Promise<T>;
export const MAX_PROFILE_PHOTO_BYTES = 2_097_152;
export class ProfileError extends Error {
  constructor(public status: number, message: string, public code = 'profile_unavailable') {
    super(message);
    this.name = 'ProfileError';
  }
}
const record = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value);
const revision = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0;
const text = (value: unknown, limit = 2000): value is string => typeof value === 'string' && [...value].length <= limit && ![...value].some(c => c.codePointAt(0)! < 32 || c.codePointAt(0) === 127);
const invalid = () => new ProfileError(502, 'The profile response could not be verified. Reload your profile.');
function parsePhoto(value: unknown): PhotoResult {
  if (!record(value) || !revision(value.revision) || typeof value.has_photo !== 'boolean' ||
    (value.photo_url !== null && value.photo_url !== '/api/v1/profile/photo') || (!value.has_photo && value.photo_url !== null)) throw invalid();
  return { revision: value.revision, has_photo: value.has_photo, photo_url: value.photo_url };
}
function parseProfile(value: unknown): CitizenProfile {
  if (!record(value) || !text(value.full_name) || !text(value.nic) || !text(value.mobile_number) ||
    !(value.email === null || text(value.email)) || !text(value.name_si, 200) || !text(value.name_ta, 200) ||
    !text(value.date_of_birth, 10) || !['', 'male', 'female'].includes(String(value.gender)) ||
    !record(value.address) || !text(value.address.line1, 200) || !text(value.address.line2, 200) || !text(value.address.city, 100)) throw invalid();
  return {
    full_name: value.full_name, nic: value.nic, mobile_number: value.mobile_number, email: value.email,
    name_si: value.name_si, name_ta: value.name_ta, date_of_birth: value.date_of_birth,
    gender: value.gender as ProfileFields['gender'], address: { line1: value.address.line1, line2: value.address.line2, city: value.address.city }, ...parsePhoto(value)
  };
}
const safeCodes = new Set(['revision_conflict', 'dob_nic_mismatch', 'invalid_gender', 'photo_too_large', 'unsupported_photo_type']);
async function failure(response: Response): Promise<never> {
  const value: unknown = await boundedJson(response, 65536).catch(() => null);
  const code = record(value) && typeof value.code === 'string' && safeCodes.has(value.code) ? value.code : 'profile_unavailable';
  const messages: Record<number, string> = {
    400: 'Check your profile fields and reload before saving again.',
    401: 'Your session ended. Please log in again.',
    403: 'This profile request was not permitted.',
    404: 'This profile feature or photo is not available.',
    409: 'Your profile changed elsewhere. Load the latest profile and review your edits.',
    413: 'Choose a JPEG or PNG photo up to 2 MiB, or shorten your profile details.',
    415: 'Choose a valid JPEG or PNG photo.',
    422: code === 'dob_nic_mismatch' ? 'Your date of birth must match your registered NIC.' : 'Check your names, address, date of birth and gender.',
    428: 'Load your latest profile before uploading a photo.',
    429: 'Too many requests. Please wait before trying again.',
  };
  throw new ProfileError(response.status, messages[response.status] ?? 'The profile request could not be confirmed. Reload to check whether your change was saved.', code);
}
export function editableProfile(profile: CitizenProfile): ProfileFields {
  return { name_si: profile.name_si, name_ta: profile.name_ta, date_of_birth: profile.date_of_birth, gender: profile.gender, address: { ...profile.address } };
}
export function validateProfile(fields: ProfileFields): string {
  for (const [value, limit] of [[fields.name_si, 200], [fields.name_ta, 200], [fields.address.line1, 200], [fields.address.line2, 200], [fields.address.city, 100]] as const) {
    if (!text(value, limit)) return 'Names and address lines allow 200 characters; city allows 100. Remove control characters.';
  }
  const date = fields.date_of_birth;
  if (date) {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || date < '0001-01-01' || date > new Date().toISOString().slice(0, 10)) return 'Enter a valid date of birth as YYYY-MM-DD.';
    const parsed = new Date(date + 'T00:00:00Z');
    if (!Number.isFinite(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== date) return 'Enter a real calendar date as YYYY-MM-DD.';
  }
  if (!['', 'male', 'female'].includes(fields.gender)) return 'Choose Male or Female.';
  return '';
}
export function photoContentType(bytes: Uint8Array): ProfilePhoto['contentType'] {
  if (bytes.length > MAX_PROFILE_PHOTO_BYTES || !bytes.length) throw new ProfileError(413, 'Choose a photo up to 2 MiB.');
  if (bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) return 'image/jpeg';
  if ([137, 80, 78, 71, 13, 10, 26, 10].every((value, index) => bytes[index] === value)) return 'image/png';
  throw new ProfileError(415, 'Choose a valid JPEG or PNG photo.');
}
function base64(bytes: Uint8Array): string {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  const chunks: string[] = [];
  let chunk = '';
  for (let offset = 0; offset < bytes.length; offset += 3) {
    const a = bytes[offset], b = bytes[offset + 1], c = bytes[offset + 2];
    chunk += alphabet[a >> 2] + alphabet[((a & 3) << 4) | ((b ?? 0) >> 4)] +
      (offset + 1 < bytes.length ? alphabet[((b & 15) << 2) | ((c ?? 0) >> 6)] : '=') +
      (offset + 2 < bytes.length ? alphabet[c & 63] : '=');
    if (chunk.length >= 16384) { chunks.push(chunk); chunk = ''; }
  }
  if (chunk) chunks.push(chunk);
  return chunks.join('');
}
// The screen receives this gateway, never an access token. Writes are not retried.
export function createProfileApi(getBase: () => string, withAccess: ProfileAccess, transport: typeof fetch = fetch) {
  async function request<T>(path: '/api/v1/profile' | '/api/v1/profile/photo', options: RequestInit, read: (response: Response) => Promise<T>, signal?: AbortSignal): Promise<T> {
    if (signal?.aborted) throw new Error('Profile request cancelled.');
    return withAccess(async token => {
      if (!token || token.length > 16384 || /\s/.test(token)) throw new ProfileError(401, 'Please log in again.');
      const base = validateApiBase(getBase());
      const controller = new AbortController();
      const abort = () => controller.abort();
      signal?.addEventListener('abort', abort, { once: true });
      if (signal?.aborted) controller.abort();
      const timer = setTimeout(abort, 30000);
      try {
        const response = await transport(base + path, {
          ...options, credentials: 'omit', cache: 'no-store', redirect: 'error', signal: controller.signal,
          headers: { ...options.headers, Authorization: `Bearer ${token}` }
        });
        if (!response.ok) return await failure(response);
        return await read(response);
      } catch (cause) {
        if (cause instanceof ProfileError) throw cause;
        throw new ProfileError(503, 'The profile request could not be confirmed. Reload to check whether your change was saved.');
      } finally { clearTimeout(timer); signal?.removeEventListener('abort', abort); }
    });
  }
  return {
    load(signal?: AbortSignal) {
      return request('/api/v1/profile', {}, async response => parseProfile(await boundedJson(response, 65536)), signal);
    },
    save(fields: ProfileFields, currentRevision: number, signal?: AbortSignal) {
      if (!revision(currentRevision) || currentRevision >= Number.MAX_SAFE_INTEGER) return Promise.reject(new ProfileError(400, 'Reload your profile before saving.'));
      const body = JSON.stringify({
        revision: currentRevision, name_si: fields.name_si, name_ta: fields.name_ta, date_of_birth: fields.date_of_birth, gender: fields.gender,
        address: { line1: fields.address.line1, line2: fields.address.line2, city: fields.address.city }
      });
      if (utf8Length(body) > 16384) return Promise.reject(new ProfileError(413, 'Your profile details are too long.'));
      return request('/api/v1/profile', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body }, async response => parseProfile(await boundedJson(response, 65536)), signal);
    },
    photo(signal?: AbortSignal) {
      return request('/api/v1/profile/photo', {}, async response => {
        const type = response.headers.get('Content-Type')?.split(';')[0].trim().toLowerCase();
        if (type !== 'image/jpeg' && type !== 'image/png') throw invalid();
        const length = Number(response.headers.get('Content-Length'));
        if (length > MAX_PROFILE_PHOTO_BYTES) throw invalid();
        const bytes = new Uint8Array(await response.arrayBuffer());
        if (photoContentType(bytes) !== type) throw invalid();
        return `data:${type};base64,${base64(bytes)}`;
      }, signal);
    },
    upload(photo: ProfilePhoto, currentRevision: number, signal?: AbortSignal) {
      if (!revision(currentRevision) || currentRevision >= Number.MAX_SAFE_INTEGER) return Promise.reject(new ProfileError(400, 'Reload your profile before uploading.'));
      if (photoContentType(new Uint8Array(photo.data)) !== photo.contentType) return Promise.reject(new ProfileError(415, 'Choose a valid JPEG or PNG photo.'));
      return request('/api/v1/profile/photo', { method: 'PUT', headers: { 'Content-Type': photo.contentType, 'If-Match': `"${currentRevision}"` }, body: photo.data }, async response => {
        const result = parsePhoto(await boundedJson(response, 65536));
        if (!result.has_photo || result.photo_url !== '/api/v1/profile/photo') throw invalid();
        return result;
      }, signal);
    },
  };
}
export type ProfileApi = ReturnType<typeof createProfileApi>;
