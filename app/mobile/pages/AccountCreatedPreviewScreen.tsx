import { AccountCreatedContent } from '../components/AccountCreatedContent';

export function AccountCreatedPreviewScreen({ onExit }: { onExit: () => void }) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW !== '1') return null;
  return <AccountCreatedContent mode="standalone" onExit={onExit} />;
}

export function AccountCreatedDemoScreen({ onContinue }: { onContinue: () => void }) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO !== '1') return null;
  return <AccountCreatedContent mode="demo" onContinue={onContinue} />;
}
