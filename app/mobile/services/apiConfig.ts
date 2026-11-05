export type MobileApiConfig = { authBaseUrl: string; coreBaseUrl?: string };

export function validateApiBase(value: string | undefined): string {
  if (!value || value.length > 512) throw new Error('Configure a phone-accessible HTTPS API address.');
  let url: URL;
  try { url = new URL(value); } catch { throw new Error('Invalid mobile API configuration.'); }
  if (url.protocol !== 'https:' || !url.hostname || url.username || url.password || url.search || url.hash || url.pathname !== '/' || ['localhost', '127.0.0.1', '[::1]', '::1'].includes(url.hostname.toLowerCase())) {
    throw new Error('Use a phone-accessible HTTPS API origin, without credentials or a path.');
  }
  return url.origin;
}
// Expo only inlines direct EXPO_PUBLIC references. These are public addresses,
// never Keycloak client secrets, database credentials or signing keys.
export function loadMobileApiConfig(): MobileApiConfig {
  return {
    authBaseUrl: validateApiBase(process.env.EXPO_PUBLIC_AUTH_API_URL),
    ...(process.env.EXPO_PUBLIC_CORE_API_URL ? { coreBaseUrl: validateApiBase(process.env.EXPO_PUBLIC_CORE_API_URL) } : {}),
  };
}
