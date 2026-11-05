import { validateApiBase } from './apiConfig';
export type NativeTokens = { accessToken: string; refreshToken: string; expiresIn: number; refreshExpiresIn: number };
export class NativeAuthError extends Error {
  status: number;
  constructor(status: number) {
    super(status === 401 ? 'Invalid credentials, unverified account or expired session.' : status === 429 ? 'Too many attempts. Please wait before trying again.' : 'Authentication could not be confirmed. Please try again later.');
    this.status = status;
  }
}
export function createNativeAuthApi(apiBase: string, transport: typeof fetch = fetch) {
  const base = validateApiBase(apiBase);
  async function request(path: string, body: unknown, expected: number): Promise<unknown> {
    let response: Response;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 15000);
    try {
      response = await transport(base + '/api/v1/auth/' + path, {
        method: 'POST', credentials: 'omit', cache: 'no-store', redirect: 'error', signal: controller.signal,
        headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
      });
      if (response.status !== expected) throw new NativeAuthError(response.status);
      if (expected === 204) return undefined;
      const text = await response.text();
      if (text.length > 65536) throw new NativeAuthError(502);
      try { return JSON.parse(text); } catch { throw new NativeAuthError(502); }
    } catch (error) { throw error instanceof NativeAuthError ? error : new NativeAuthError(503); }
    finally { clearTimeout(timer); }
  }
  function tokens(value: unknown): NativeTokens {
    const v = value as Record<string, unknown> | null;
    const validToken = (t: unknown): t is string => typeof t === 'string' && t.length > 0 && t.length <= 16384 && !/\s/.test(t);
    if (!v || v.token_type !== 'Bearer' || !validToken(v.access_token) || !validToken(v.refresh_token) || typeof v.expires_in !== 'number' || !Number.isInteger(v.expires_in) || v.expires_in < 1 || v.expires_in > 3600 || typeof v.refresh_expires_in !== 'number' || !Number.isInteger(v.refresh_expires_in) || v.refresh_expires_in < 1 || v.refresh_expires_in > 31536000) throw new NativeAuthError(502);
    return { accessToken: v.access_token, refreshToken: v.refresh_token, expiresIn: v.expires_in, refreshExpiresIn: v.refresh_expires_in };
  }
  return {
    async login(identifier: string, password: string) {
      if (!identifier.trim() || !password) throw new NativeAuthError(400);
      return tokens(await request('login', { identifier: identifier.trim(), password }, 200));
    },
    async refresh(refreshToken: string) { return tokens(await request('refresh', { refresh_token: refreshToken }, 200)); },
    async revoke(refreshToken: string) { await request('revoke', { refresh_token: refreshToken }, 204); },
  };
}
