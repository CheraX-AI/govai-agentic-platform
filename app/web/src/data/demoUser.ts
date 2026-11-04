// Synthetic display details for layout previews only; not an authenticated user.
const displayName = 'Harshan'

export const demoUser = {
  displayName,
  initials: displayName.split(/\s+/).map(part => part[0]).join('').toUpperCase(),
}
