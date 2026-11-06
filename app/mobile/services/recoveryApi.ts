import { apiRequest, boundedJson, utf8Length } from './publicRequest';
export type RecoveryReceipt = { nic: string; maskedEmail?: string };
export async function requestRecovery(base: string, nic: string, transport: typeof fetch = fetch): Promise<RecoveryReceipt> {
 const normalized = nic.trim().toUpperCase();
 if (!/^(?:[0-9]{9}[VX]|[0-9]{12})$/.test(normalized)) throw new Error('Please check your NIC number.');
 let response: Response;
 try { response = await apiRequest(base,'/api/v1/auth/forgot-password',{nic:normalized},{},transport); }
 catch { throw new Error('Unable to confirm delivery. Check your email before requesting another link.'); }
 if (response.status===429) throw new Error('Please wait before requesting another link.');
 if (response.status===400||response.status===422) throw new Error('Please check your NIC number.');
 if (response.status!==202) throw new Error('Recovery is temporarily unavailable. Please try again later.');
 const result=await boundedJson(response,4096).catch(()=>null);
 if(result?.status!=='received'||(result.masked_email!==undefined&&(typeof result.masked_email!=='string'||result.masked_email.length>254))) throw new Error('Unable to confirm the recovery request.');
 return {nic:normalized,maskedEmail:result.masked_email};
}
export function validResetToken(token: string) { return /^[A-Za-z0-9_-]{43}$/.test(token); }
export function parseResetLink(value: string, approvedHttpsResetUrl?: string): string | null {
 // Accept only this app's dedicated reset route. Never interpret arbitrary links as reset tokens.
 try {
  const url=new URL(value);
  const route=url.protocol==='govai:' ? url.hostname+url.pathname : url.pathname;
  let trustedHttps = false;
  if (approvedHttpsResetUrl) {
    const approved = new URL(approvedHttpsResetUrl);
    trustedHttps = approved.protocol === 'https:' && !approved.username && !approved.password && !approved.search && !approved.hash &&
      url.protocol === 'https:' && url.origin === approved.origin && url.pathname === approved.pathname;
  }
  const allowed = trustedHttps || url.protocol==='govai:' && (route==='reset-password'||route==='reset-password/') ||
    (url.protocol==='exp:'||url.protocol==='exps:') && url.pathname==='/--/reset-password';
  const token=url.hash.startsWith('#token=')?url.hash.slice(7):'';
  return allowed&&!url.username&&!url.password&&!url.search&&validResetToken(token)?token:null;
 } catch { return null; }
}
export async function resetPassword(base: string, token: string, password: string, confirmation: string, transport: typeof fetch = fetch): Promise<void> {
 if(!validResetToken(token)) throw new Error('This reset link is invalid. Request a new link.');
 if([...password].length<8||utf8Length(password)>1024||password!==confirmation) throw new Error('Use at least 8 characters and matching passwords.');
 let response: Response;
 try { response=await apiRequest(base,'/api/v1/auth/reset-password',{token,new_password:password,confirm_password:confirmation},{},transport); }
 catch { throw new Error('Unable to confirm the password change. Try logging in or request a new link.'); }
 if(response.status===204)return;
 if(response.status===429)throw new Error('Too many attempts. Please wait.');
 if(response.status===400||response.status===422)throw new Error('Check the password requirements. This link may be invalid, expired or already used.');
 throw new Error('Unable to confirm the password change. Try logging in or request a new link.');
}
