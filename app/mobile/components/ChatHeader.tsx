import type { RefObject } from 'react';
import { Image, Pressable, StyleSheet, Text, View } from 'react-native';
import { conversationAssets } from '../chatDemo';
import { layout } from '../layout';

export function ChatIcon({ label, asset, size, tone, onPress, disabled = !onPress, expanded, controlRef }: {
  label: string; asset: number; size: number; tone?: 'muted' | 'brand' | 'subtle' | 'border';
  onPress?: () => void; disabled?: boolean; expanded?: boolean; controlRef?: RefObject<View | null>;
}) {
  return <Pressable ref={controlRef} onPress={onPress} disabled={disabled} accessibilityRole="button"
    accessibilityLabel={label} accessibilityState={{ disabled, ...(expanded === undefined ? {} : { expanded }) }} style={controlStyles.iconTarget}>
    <View style={[controlStyles.iconDisc, tone && controlStyles[tone]]}>
      <Image source={asset} accessible={false} resizeMode="contain" style={{ width: size, height: size }} />
    </View>
  </Pressable>;
}
export function ChatHeader({ title, onBack, onNewChat, onMenu, menuRef, ready, menuVisible, menuAsset, notificationAsset }: {
  title?: string; onBack: () => void; onNewChat: () => void; onMenu: () => void; menuRef: RefObject<View | null>;
  ready: boolean; menuVisible: boolean; menuAsset: number; notificationAsset: number;
}) {
  return <View style={styles.header}>
    {title ? <ChatIcon label="Back to Chat Home" asset={conversationAssets.back} size={20} tone="muted" onPress={onBack} />
      : <ChatIcon label="Open menu" asset={menuAsset} size={20} tone="border" onPress={onMenu} disabled={!ready} expanded={menuVisible} controlRef={menuRef} />}
    {title ? <View style={styles.title}><Text accessibilityRole="header" style={styles.conversationTitle}>{title}</Text>
      <View style={styles.subtitle}><Image source={conversationAssets.status} accessible={false} style={{ width: 7, height: 7 }} /><Text style={styles.caption}>Local mock assistant</Text></View>
    </View> : <Text accessibilityRole="header" style={styles.brandName}>GovAI</Text>}
    {title ? <ChatIcon label="New chat" asset={conversationAssets.newChat} size={20} tone="muted" onPress={onNewChat} />
      : <ChatIcon label="Notifications. Unavailable in this development preview." asset={notificationAsset} size={20} tone="subtle" />}
  </View>;
}
export const controlStyles = StyleSheet.create({
  iconTarget: { width: layout.touchTarget, minHeight: layout.touchTarget, alignItems: 'center', justifyContent: 'center', flexShrink: 0 },
  iconDisc: { width: 40, height: 40, borderRadius: 9999, alignItems: 'center', justifyContent: 'center' },
  muted: { backgroundColor: '#f5f5f5' }, brand: { backgroundColor: '#a30f2d' }, subtle: { backgroundColor: '#fcf3f5' }, border: { borderWidth: 1, borderColor: '#e5e7eb' },
});
const styles = StyleSheet.create({
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: 12, paddingVertical: 4, gap: 8 },
  brandName: { flexShrink: 1, fontFamily: 'Inter_700Bold', fontSize: 18, lineHeight: 24, color: '#142033', textAlign: 'center' },
  title: { flex: 1, alignItems: 'center', gap: 2 }, conversationTitle: { fontFamily: 'Inter_700Bold', fontSize: 15, lineHeight: 21, color: '#142033', textAlign: 'center' },
  subtitle: { flexDirection: 'row', alignItems: 'center', gap: 6 }, caption: { fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 17, color: '#666', flexShrink: 1 },
});
