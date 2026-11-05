import { validateApiBase } from './apiConfig';
export function utf8Length(value: string): number {
  let size = 0;
  for (const character of value) { const point = character.codePointAt(0)!; size += point <= 127 ? 1 : point <= 2047 ? 2 : point <= 65535 ? 3 : 4; }
  return size;
}
export async function apiRequest(base: string, path: string, body: unknown, options: { token?: string; signal?: AbortSignal; timeout?: number } = {}, transport: typeof fetch = fetch): Promise<Response> {
  const origin = validateApiBase(base);
  const controller = new AbortController();
  const abort = () => controller.abort();
  options.signal?.addEventListener('abort', abort, { once: true });
  if (options.signal?.aborted) controller.abort();
  const timer = setTimeout(abort, options.timeout ?? 35000);
  try {
    const response = await transport(origin + path, { method: 'POST', credentials: 'omit', redirect: 'error', cache: 'no-store',
      headers: { 'Content-Type': 'application/json', ...(options.token ? { Authorization: `Bearer ${options.token}` } : {}) },
      body: JSON.stringify(body), signal: controller.signal });
    // Keep cancellation/deadline active through body reception, not just headers.
    const text = await response.text();
    if (utf8Length(text) > 65536) throw new Error('Invalid API response.');
    return new Response(response.status === 204 ? null : text, { status: response.status, headers: response.headers });
  } finally { clearTimeout(timer); options.signal?.removeEventListener('abort', abort); }
}
export async function boundedJson(response: Response, limit: number): Promise<any> {
  const text = await response.text();
  if (utf8Length(text) > limit) throw new Error('Invalid API response.');
  return JSON.parse(text);
}
