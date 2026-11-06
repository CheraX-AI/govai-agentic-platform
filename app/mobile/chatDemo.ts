import type { ChatSource } from './services/chatApi';
export type ChatMessage = {
  id: number;
  role: 'citizen' | 'ai';
  text: string;
  nicSample?: boolean;
  live?: boolean;
  sources?: ChatSource[];
};
export type LocalThread = { id: number; title: string; messages: ChatMessage[] };
export const conversationAssets = {
  back: require('./assets/figma/chat-conversation/back.png'),
  newChat: require('./assets/figma/chat-conversation/new-chat.png'),
  avatar: require('./assets/figma/chat-conversation/avatar.png'),
  status: require('./assets/figma/chat-conversation/status.png'),
  idCard: require('./assets/figma/chat-conversation/id-card.png'),
  arrow: require('./assets/figma/chat-conversation/arrow.png'),
  globe: require('./assets/figma/chat-conversation/globe.png'),
  profile: require('./assets/figma/chat-conversation/profile-bell.png'),
};
export function localAnswer(prompt: string): Pick<ChatMessage, 'text' | 'nicSample'> {
  const nicSample = prompt.trim() === 'How do I renew my NIC?';
  return {
    nicSample,
    text: nicSample
      ? 'This is a local sample of the NIC renewal conversation. No government process, fee or timeframe has been verified.'
      : 'This is a local mock response. No AI service was contacted, and no government guidance was verified.',
  };
}
