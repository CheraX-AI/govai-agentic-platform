import { createNativeAuthApi } from './nativeAuthApi';
import { createNativeSession } from './nativeSession';
import * as SecureStore from 'expo-secure-store';
import { createRefreshStorage } from './refreshStorage';
export async function openSecureSessionStorage(authApiBase: string) {
  if (!(await SecureStore.isAvailableAsync())) throw new Error('Secure session storage is unavailable on this device.');
  return createRefreshStorage(authApiBase, {
    getItemAsync: key => SecureStore.getItemAsync(key),
    setItemAsync: (key, value) => SecureStore.setItemAsync(key, value),
    deleteItemAsync: key => SecureStore.deleteItemAsync(key),
  });
}

export async function openNativeSession(authApiBase: string) {
  const store = await openSecureSessionStorage(authApiBase);
  return createNativeSession(createNativeAuthApi(authApiBase), store);
}
