import { apiRequest, boundedJson, utf8Length } from './publicRequest';
export type ChatSource = { number: number; title: string; page?: number; url?: string };
// conversationId is the backend's ID for the open chat. It is never shown in the UI.
export type ChatReply = { answer: string; sources: ChatSource[]; conversationId?: string };
export class ChatError extends Error {
 constructor(public status: number, message: string) { super(message); }
}
// The backend's conversation ID format: a lowercase UUID (backend/pkg/chatcontract).
const CONVERSATION_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
export function isConversationId(value: unknown): value is string { return typeof value === 'string' && CONVERSATION_ID.test(value); }
// Leave conversationId out for the first message; core-service creates the conversation.
export async function sendChat(base: string, token: string, message: string, signal: AbortSignal, transport: typeof fetch = fetch, conversationId?: string): Promise<ChatReply> {
 const text = message.trim();
 if (conversationId !== undefined && !isConversationId(conversationId)) throw new ChatError(400, 'Unable to continue this conversation. Start a new chat.');
 const payload = conversationId ? {message:text, conversation_id:conversationId} : {message:text};
 if (!text || [...text].length > 4000 || utf8Length(JSON.stringify(payload)) > 16384) throw new ChatError(400, 'Enter a question of at most 4000 characters.');
 if (!token || /\s/.test(token) || token.length > 16384) throw new ChatError(401,'Please log in again.');
 let response: Response;
 try { response = await apiRequest(base, '/api/chat/message', payload, {token,signal,timeout:40000}, transport); }
 catch { throw new ChatError(503,'Unable to confirm the chat response. Please try again later.'); }
 if (!response.ok) {
  const status=response.status;
  throw new ChatError(status,status===401?'Your session ended. Please log in again.':status===403?'This chat request was not permitted.':status===400||status===413?'Check your question length and format.':status===429?'Too many requests. Please wait.':status===504?'The chat request timed out. Please try again later.':'Chat is temporarily unavailable. Please try again later.');
 }
 const reply = await boundedJson(response, 65536).catch(()=>null)
 if(typeof reply?.answer !== 'string' || !reply.answer.trim() || utf8Length(reply.answer) > 32768 || !Array.isArray(reply.sources) || reply.sources.length > 20) throw new ChatError(502,'Unable to retrieve a valid answer.')
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
 // The backend owns the ID: it must be well formed, and a continued chat must keep its ID.
 const returned = reply.conversation_id
 if((returned !== undefined && !isConversationId(returned)) || (conversationId && returned !== conversationId)) throw new ChatError(502,'Unable to retrieve a valid answer.')
 return {answer:reply.answer,sources,...(returned !== undefined ? {conversationId:returned} : {})}
}
