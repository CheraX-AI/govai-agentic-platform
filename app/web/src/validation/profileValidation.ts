import { MAX_PHOTO_BYTES } from '../services/profileApi'
import type { ProfileFields } from '../services/profileApi'

// Early form feedback only; the backend remains authoritative for NIC/DOB,
// ownership, image contents and all permitted-field rules.
export function validateProfile(fields: ProfileFields): string {
  const limits = [
    [fields.name_si, 200],
    [fields.name_ta, 200],
    [fields.address.line1, 200],
    [fields.address.line2, 200],
    [fields.address.city, 100],
  ] as const

  for (const [value, limit] of limits) {
    const characters = [...value]
    const hasControlCharacter = characters.some((character) => {
      const code = character.codePointAt(0)!
      return code < 32 || code === 127
    })
    if (characters.length > limit || hasControlCharacter) {
      return 'Names and address lines allow 200 characters; city allows 100. Remove control characters.'
    }
  }
  return ''
}

export function validateProfilePhoto(photo: Blob): string {
  if (!['image/jpeg', 'image/png'].includes(photo.type) || !photo.size) {
    return 'Choose a valid JPEG or PNG photo.'
  }
  if (photo.size > MAX_PHOTO_BYTES) return 'Choose a photo up to 2 MiB.'
  return ''
}
