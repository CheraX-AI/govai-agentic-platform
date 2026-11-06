import { NativeAuthError } from './nativeAuthApi';
import type { NativeTokens } from './nativeAuthApi';
import type { RefreshCredential } from './refreshStorage';
export type NativeSessionApi = {
  login(identifier: string, password: string): Promise<NativeTokens>;
  refresh(token: string): Promise<NativeTokens>;
  revoke(token: string): Promise<void>;
};
export type RefreshStore = { load(): Promise<RefreshCredential | null>; save(value: RefreshCredential): Promise<void>; clear(): Promise<void> };
// Access tokens are closure-local, never saved. Refresh is serialized and the
// old persisted credential is consumed BEFORE network access. Unknown refresh
// outcomes require a new login, rather than replaying a single-use token.
export function createNativeSession(api: NativeSessionApi, store: RefreshStore, now = Date.now) {
  let access: { token: string; until: number } | null = null;
  let queue: Promise<unknown> = Promise.resolve();
  function serial<T>(work: () => Promise<T>): Promise<T> {
    const result = queue.then(work, work); queue = result.catch(() => undefined); return result;
  }
  async function install(issued: NativeTokens, receivedAt: number) {
    const accessUntil = receivedAt + issued.expiresIn * 1000;
    const refreshUntil = receivedAt + issued.refreshExpiresIn * 1000;
    try {
      if (accessUntil <= now() || refreshUntil <= now()) throw new Error('Session expired during authentication.');
      await store.save({ token: issued.refreshToken, expiresAt: refreshUntil });
    } catch {
      access = null;
      try { await store.clear(); } catch { /* No authenticated state is published. */ }
      try { await api.revoke(issued.refreshToken); } catch { /* Never retry uncertain provider mutation. */ }
      throw new Error('Unable to save a secure session. Please log in again.');
    }
    access = { token: issued.accessToken, until: accessUntil };
  }
  async function ensureAccess(): Promise<string | null> {
    if (access && access.until > now() + 30000) return access.token;
    access = null;
    const saved = await store.load();
    if (!saved) return null;
    await store.clear();
    if (saved.expiresAt <= now()) return null;
    const started = now();
    const issued = await api.refresh(saved.token);
    await install(issued, started);
    return access!.token;
  }
  return {
    login: (identifier: string, password: string) => serial(async () => {
      if (access || await store.load()) throw new Error('Sign out before starting another session.');
      const started = now();
      const issued = await api.login(identifier, password);
      await install(issued, started);
    }),
    restore: () => serial(async () => Boolean(await ensureAccess())),
    async withAccess<T>(send: (token: string) => Promise<T>): Promise<T> {
      const token = await serial(ensureAccess);
      if (!token) throw new NativeAuthError(401);
      return send(token);
    },
    logout: () => serial(async () => {
      access = null;
      const saved = await store.load();
      await store.clear();
      if (!saved) return;
      try { await api.revoke(saved.token); }
      catch (error) { if (!(error instanceof NativeAuthError && error.status === 401)) throw new Error('Signed out locally; provider revocation could not be confirmed.'); }
    }),
  };
}
