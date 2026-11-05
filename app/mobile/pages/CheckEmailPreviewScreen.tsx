import { CheckEmailContent } from '../components/CheckEmailContent';

type Props = { onBack: () => void; onDifferentNic: () => void; onLogin: () => void; onExit: () => void };
export function CheckEmailPreviewScreen({ onBack, onDifferentNic, onLogin }: Props) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW !== '1') return null;
  return <CheckEmailContent mode="standalone" onBack={onBack} onDifferentNic={onDifferentNic} onLogin={onLogin} />;
}

export function CheckEmailDemoScreen({ onBack, onDifferentNic, onLogin }: Omit<Props, 'onExit'>) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO !== '1') return null;
  return <CheckEmailContent mode="demo" onBack={onBack} onDifferentNic={onDifferentNic} onLogin={onLogin} />;
}
