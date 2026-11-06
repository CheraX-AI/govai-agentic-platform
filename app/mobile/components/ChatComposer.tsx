import type { RefObject } from 'react';
import { StyleSheet, Text, TextInput, View } from 'react-native';
import { ChatIcon } from './ChatHeader';
import { layout } from '../layout';
export function ChatComposer({
  inputRef,
  value,
  onChange,
  onSend,
  onFocus,
  onBlur,
  onLayout,
  onContentSizeChange,
  blocked,
  pending,
  conversation,
  error,
  assets,
}: {
  inputRef: RefObject<TextInput | null>;
  value: string;
  onChange: (text: string) => void;
  onSend: () => void;
  onFocus: () => void;
  onBlur: () => void;
  onLayout: () => void;
  onContentSizeChange: () => void;
  blocked: boolean;
  pending: boolean;
  conversation: boolean;
  error: string;
  assets: { plus: number; mic: number; send: number };
}) {
  return (
    <View onLayout={onLayout} style={styles.composerArea}>
      <View style={styles.composer}>
        <ChatIcon
          label="Attach. Unavailable in this development preview."
          asset={assets.plus}
          size={20}
          tone="muted"
        />
        <TextInput
          ref={inputRef}
          value={value}
          onChangeText={onChange}
          editable={!blocked}
          multiline
          placeholder={conversation ? 'Ask a follow-up question' : 'Ask GovAI anything'}
          placeholderTextColor="#666666"
          accessibilityLabel="Message draft"
          accessibilityHint="Local demo only. Send prepares a mock response. Drafts are cleared when you exit."
          onFocus={onFocus}
          onBlur={onBlur}
          onContentSizeChange={onContentSizeChange}
          style={styles.input}
        />
        <ChatIcon
          label="Microphone. Unavailable in this development preview."
          asset={assets.mic}
          size={17}
        />
        <ChatIcon
          label="Send local message"
          asset={assets.send}
          size={17}
          tone="brand"
          onPress={onSend}
          disabled={blocked || pending}
        />
      </View>
      {!!error && (
        <Text accessibilityRole="alert" accessibilityLiveRegion="polite" style={styles.error}>
          {error}
        </Text>
      )}
    </View>
  );
}
const styles = StyleSheet.create({
  composerArea: { paddingHorizontal: 16, paddingTop: 8, paddingBottom: 12 },
  composer: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 6,
    gap: 2,
    borderRadius: 30,
    borderWidth: 1,
    borderColor: '#f1f5f9',
    boxShadow: '0px 4px 14px rgba(23,32,51,0.07)',
  },
  input: {
    flex: 1,
    minWidth: 0,
    minHeight: layout.touchTarget,
    fontFamily: 'Inter_400Regular',
    fontSize: 13,
    lineHeight: 20,
    color: '#142033',
    paddingHorizontal: 2,
    paddingVertical: 12,
  },
  error: {
    fontFamily: 'Inter_400Regular',
    fontSize: 13,
    lineHeight: 19,
    color: '#a30f2d',
    paddingTop: 4,
  },
});
