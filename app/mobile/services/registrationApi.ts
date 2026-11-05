import { validateApiBase } from './apiConfig';

export type RegistrationPayload = { full_name: string; nic: string; mobile_number: string; email: string; password: string };

export async function registerCitizen(base: string, payload: RegistrationPayload, operation: string, transport: typeof fetch = fetch): Promise<void> {
  const origin = validateApiBase(base);
  if (!/^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i.test(operation)) throw new Error('Invalid registration request identifier.');
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 35000);
  try {
    const response = await transport(`${origin}/api/v1/registrations`, {
      method: 'POST', credentials: 'omit', redirect: 'error', cache: 'no-store',
      headers: { 'Content-Type': 'application/json', 'Idempotency-Key': operation },
      body: JSON.stringify(payload), signal: controller.signal,
    });
    if (response.status === 429) throw new Error('Too many requests. Please wait before trying again.');
    if (response.status === 400 || response.status === 422) throw new Error('Please check your registration details and password requirements.');
    if (response.status !== 202) throw new Error('Unable to confirm registration. Check your email before trying again.');
    const text = await response.text();
    if (text.length > 4096) throw new Error();
    let receipt: unknown;
    try { receipt = JSON.parse(text); } catch { throw new Error(); }
    if (!receipt || typeof receipt !== 'object' || !('status' in receipt) || receipt.status !== 'received') throw new Error();
  } catch (error) {
    if (error instanceof Error && ['Too many requests. Please wait before trying again.', 'Please check your registration details and password requirements.', 'Unable to confirm registration. Check your email before trying again.'].includes(error.message)) throw error;
    throw new Error('Unable to confirm registration. Check your email before trying again.');
  } finally { clearTimeout(timer); }
}
