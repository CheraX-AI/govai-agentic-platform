// A document cited in the answer: number matches the [n] marker in the answer text.
// page and url are optional (the documents have no public address yet).
export type ChatSource = { number: number; title: string; page?: number; url?: string }
export type ChatReply = { answer: string; sources: ChatSource[] }
export class ChatError extends Error {
 status: number
 constructor(status: number, message: string) { super(message); this.status = status }
}
export async function sendChat(message: string, signal: AbortSignal): Promise<ChatReply> {
 const text = message.trim()
 if (!text || [...text].length > 4000 || new TextEncoder().encode(JSON.stringify({message:text})).length > 16384) throw new ChatError(400, 'Enter a question of at most 4000 characters.')
 const base = new URL(import.meta.env?.VITE_AUTH_API_URL ?? 'https://localhost:5001')
 if(base.protocol !== 'https:' || base.username || base.password || base.search || base.hash || base.pathname !== '/') throw new ChatError(503,'Chat configuration is unavailable.')
 const bounded = AbortSignal.any([signal, AbortSignal.timeout(40_000)])
 async function request(path: string, options: RequestInit) {
  try { return await fetch(new URL('/api/v1/web/'+path,base),{...options,credentials:'include',cache:'no-store',redirect:'error',signal:bounded}) }
  catch { throw new ChatError(503,'Unable to confirm the chat response. Please try again later.') }
 }
 const headers = {'X-GovAI-Web':'1'}
 const csrfResponse = await request('csrf',{headers})
 if(csrfResponse.status===401) throw new ChatError(401,'Your session ended. Please log in again.')
 if(!csrfResponse.ok) throw new ChatError(503,'Chat is temporarily unavailable.')
 const csrf = await csrfResponse.json().catch(()=>null)
 if(typeof csrf?.csrf_token !== 'string' || !/^[A-Za-z0-9_-]{43}$/.test(csrf.csrf_token)) throw new ChatError(503,'Unable to confirm your session.')
 const response = await request('chat',{method:'POST',headers:{...headers,'Content-Type':'application/json','X-CSRF-Token':csrf.csrf_token},body:JSON.stringify({message:text})})
 if(!response.ok) {
  const status=response.status
  throw new ChatError(status,status===401?'Your session ended. Please log in again.':status===403?'This chat request was not permitted.':status===400||status===413?'Check your question length and format.':status===429?'Too many requests. Please wait.':status===504?'The chat request timed out. Please try again later.':'Chat is temporarily unavailable. Please try again later.')
 }
 const reply = await response.json().catch(()=>null)
 if(typeof reply?.answer !== 'string' || !reply.answer.trim() || new TextEncoder().encode(reply.answer).length > 32768 || !Array.isArray(reply.sources) || reply.sources.length > 20) throw new ChatError(502,'Unable to retrieve a valid answer.')
 const numbers = new Set<number>()
 const sources: ChatSource[] = []
 for(const source of reply.sources) {
  const invalid = () => new ChatError(502,'Unable to retrieve valid sources.')
  if(!source || typeof source !== 'object' || !Number.isInteger(source.number) || source.number<1 || source.number>100 || numbers.has(source.number)) throw invalid()
  if(typeof source.title !== 'string' || !source.title.trim() || source.title.length>512) throw invalid()
  if(source.page !== undefined && (!Number.isInteger(source.page) || source.page<1 || source.page>100000)) throw invalid()
  if(source.url !== undefined) {
   if(typeof source.url !== 'string' || source.url.length>2048 || /[\s]/.test(source.url)) throw invalid()
   let url: URL
   try {url=new URL(source.url)} catch {throw invalid()}
   if(url.protocol!=='https:'||url.username||url.password) throw invalid()
  }
  numbers.add(source.number)
  sources.push({number:source.number,title:source.title,...(source.page!==undefined?{page:source.page}:{}),...(source.url!==undefined?{url:source.url}:{})})
 }
 return {answer:reply.answer,sources}
}
