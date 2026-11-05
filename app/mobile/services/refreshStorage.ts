import { validateApiBase } from './apiConfig';
export type SecureStoragePort = {
  getItemAsync(key: string): Promise<string | null>;
  setItemAsync(key: string, value: string): Promise<void>;
  deleteItemAsync(key: string): Promise<void>;
};
export type RefreshCredential = { token: string; expiresAt: number };
const key = 'govai.native.refresh.v1';
export function createRefreshStorage(apiBase: string, storage: SecureStoragePort, now = Date.now) {
  const issuerEnvironment = validateApiBase(apiBase);
  return {
    async save(credential: RefreshCredential): Promise<void> {
      if (typeof credential.token !== 'string' || !credential.token || credential.token.length > 16384 || /[\s]/.test(credential.token) || !Number.isSafeInteger(credential.expiresAt) || credential.expiresAt <= now()) throw new Error('Invalid refresh credential.');
      try { await storage.setItemAsync(key, JSON.stringify({ version: 1, apiBase: issuerEnvironment, token: credential.token, expiresAt: credential.expiresAt })); }
      catch { throw new Error('Secure session storage is unavailable.'); }
    },
    async load(): Promise<RefreshCredential | null> {
      let raw: string | null;
      try { raw = await storage.getItemAsync(key); } catch { throw new Error('Secure session storage is unavailable.'); }
      if (!raw) return null;
      let value: unknown;
      try { value = JSON.parse(raw); } catch { await this.clear(); return null; }
      const v = value as Record<string, unknown> | null;
      if (!v || Array.isArray(v) || typeof v !== 'object' || v.version !== 1 || Object.keys(v).sort().join(',') !== 'apiBase,expiresAt,token,version' || typeof v.apiBase !== 'string' || typeof v.token !== 'string' || !v.token || v.token.length > 16384 || /[\s]/.test(v.token) || typeof v.expiresAt !== 'number' || !Number.isSafeInteger(v.expiresAt)) { await this.clear(); return null; }
      if (v.apiBase !== issuerEnvironment) return null;
      if (v.expiresAt <= now()) { await this.clear(); return null; }
      return { token: v.token, expiresAt: v.expiresAt };
    },
    async clear(): Promise<void> {
      try { await storage.deleteItemAsync(key); } catch { throw new Error('Unable to clear the secure session.'); }
    },
  };
}
