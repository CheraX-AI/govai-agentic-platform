export type RegistrationPayload = {
  full_name: string
  nic: string
  mobile_number: string
  email: string
  password: string
}

export async function registerCitizen(payload: RegistrationPayload, operation: string, baseURL = import.meta.env?.VITE_AUTH_API_URL ?? 'https://localhost:5001'): Promise<void> {
  const url = new URL(baseURL)
  if (url.protocol !== 'https:' || url.username || url.password || url.search || url.hash || url.pathname !== '/') throw new Error('Registration configuration is unavailable.')
  let response: Response
  try {
    response = await fetch(new URL('/api/v1/registrations', url), {
      method: 'POST', credentials: 'omit', cache: 'no-store', redirect: 'error',
      headers: { 'Content-Type': 'application/json', 'Idempotency-Key': operation },
      body: JSON.stringify(payload), signal: AbortSignal.timeout(35_000),
    })
  } catch {
    throw new Error('Unable to confirm registration. Please try again later; check your email before resubmitting.')
  }
  if (response.status === 429) throw new Error('Too many requests. Please wait before trying again.')
  if (response.status === 400 || response.status === 422) throw new Error('Please check your registration details and password requirements.')
  if (response.status !== 202) throw new Error('Registration is temporarily unavailable. Please try again later.')
  const receipt = await response.json().catch(() => null)
  if (receipt?.status !== 'received') throw new Error('Unable to confirm registration. Please check your email before resubmitting.')
}
