import { useEffect, useRef, useState } from 'react';
import { sendChat } from '../services/chatApi';
import { AppState } from 'react-native';
import { loadMobileApiConfig } from '../services/apiConfig';
import { openNativeSession } from '../services/secureSession';
type Status = 'disabled' | 'checking' | 'unauthenticated' | 'signing-in' | 'authenticated' | 'error';
export function useNativeSession(enabled: boolean) {
  const [status, setStatus] = useState<Status>(enabled ? 'checking' : 'disabled');
  const [error, setError] = useState('');
  const generation = useRef(0);
  const mounted = useRef(false);
  const current = useRef(status); current.current = status;
  const instance = useRef<ReturnType<typeof openNativeSession> | null>(null);
  function session() {
    if (!enabled) throw new Error('Native login is unavailable.');
    instance.current ??= openNativeSession(loadMobileApiConfig().authBaseUrl);
    return instance.current;
  }
  useEffect(() => {
    mounted.current = true;
    if (!enabled) { setStatus('disabled'); return () => { mounted.current = false; }; }
    async function restore() {
      if (current.current === 'signing-in') return;
      const version = ++generation.current;
      try {
        const available = await (await session()).restore();
        if (mounted.current && version === generation.current) { setStatus(available ? 'authenticated' : 'unauthenticated'); setError(''); }
      } catch {
        if (mounted.current && version === generation.current) { setStatus('error'); setError('Unable to restore a secure session. Please log in again.'); }
      }
    }
    void restore();
    const listener = AppState.addEventListener('change', state => { if (state === 'active') void restore(); });
    const timer = setInterval(() => { if (AppState.currentState === 'active' && current.current === 'authenticated') void restore(); }, 60000);
    return () => { mounted.current = false; generation.current++; listener.remove(); clearInterval(timer); };
  }, [enabled]);
  async function login(identifier: string, password: string) {
    const version = ++generation.current; current.current = 'signing-in'; setStatus('signing-in'); setError('');
    try {
      await (await session()).login(identifier, password);
      if (mounted.current && version === generation.current) setStatus('authenticated');
    } catch (failure) {
      if (mounted.current && version === generation.current) setStatus('unauthenticated');
      throw failure;
    }
  }
  async function logout() {
    const version = ++generation.current; current.current = 'unauthenticated'; setStatus('unauthenticated');
    try { await (await session()).logout(); if (mounted.current && version === generation.current) setError(''); }
    catch { if (mounted.current && version === generation.current) { setStatus('error'); setError('Local sign-out or provider revocation could not be fully confirmed.'); } }
  }
  async function chat(message: string, signal: AbortSignal) {
    const config = loadMobileApiConfig();
    if (!config.coreBaseUrl) throw new Error('Chat configuration is unavailable.');
    return (await session()).withAccess(token => sendChat(config.coreBaseUrl!, token, message, signal));
  }
  return { status, error, login, logout, chat };
}
