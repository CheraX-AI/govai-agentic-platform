import { ChatHomeContent } from './components/ChatHomeContent';

export function ChatHomePreviewScreen({ onExit }: { onExit: () => void }) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_CHAT_HOME_PREVIEW !== '1') return null;
  return <ChatHomeContent mode="standalone" onExit={onExit} />;
}

export function ChatHomeDemoScreen({ onExit }: { onExit: () => void }) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO !== '1') return null;
  return <ChatHomeContent mode="demo" onExit={onExit} />;
}
