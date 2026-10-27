import { CheckEmailContent } from './components/CheckEmailContent';

type Props = { onBack: () => void; onDifferentNic: () => void; onExit: () => void };
export function CheckEmailPreviewScreen({ onBack, onDifferentNic }: Props) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW !== '1') return null;
  return <CheckEmailContent mode="standalone" onBack={onBack} onDifferentNic={onDifferentNic} />;
}

export function CheckEmailDemoScreen({ onBack, onDifferentNic }: Omit<Props, 'onExit'>) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO !== '1') return null;
  return <CheckEmailContent mode="demo" onBack={onBack} onDifferentNic={onDifferentNic} />;
}
