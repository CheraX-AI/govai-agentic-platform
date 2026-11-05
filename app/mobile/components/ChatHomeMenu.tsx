import { useState, type RefObject } from 'react';
import { Image, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { layout } from '../layout';

export const chatHomeMenuAssets = {
  emblem: require('../assets/figma/chat-home-menu/emblem.png'),
  wordmark: require('../assets/figma/chat-home-menu/wordmark.png'),
  close: require('../assets/figma/chat-home-menu/close.png'),
  newChat: require('../assets/figma/chat-home-menu/new-chat.png'),
  recent: require('../assets/figma/chat-home-menu/recent.png'),
  applications: require('../assets/figma/chat-home-menu/applications.png'),
  chevron: require('../assets/figma/chat-home-menu/chevron.png'),
  history: require('../assets/figma/chat-home-menu/history.png'),
  more: require('../assets/figma/chat-home-menu/more.png'),
  profile: require('../assets/figma/chat-home/profile.png'),
  settings: require('../assets/figma/chat-home-menu/settings.png'),
};
const groups = [
  { label: 'Today', titles: ['Passport application process', 'Renew a driving licence', 'Birth certificate requirements'] },
  { label: 'Yesterday', titles: ['Register a vehicle', 'Land ownership records'] },
];
type Props = { authenticated?: boolean; onClose: () => void; onCloseLayout: () => void; closeRef: RefObject<View | null>; onLogout?: () => void; onNewChat?: () => void };

// Presentation only: the parent owns visibility, Back, focus and the retained draft.
export function ChatHomeMenu({ authenticated = false, onClose, onCloseLayout, closeRef, onLogout, onNewChat }: Props) {
  const { width, height } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const [drawerHeight, setDrawerHeight] = useState<number | null>(null);
  const [headerHeight, setHeaderHeight] = useState(0);
  const [footerHeight, setFooterHeight] = useState(0);
  const topPadding = insets.top + 4;
  const bottomPadding = insets.bottom + 8;
  // Measure intrinsic fixed regions, not the flexible middle, to avoid feedback.
  const wholeDrawerScroll = headerHeight + footerHeight + topPadding + bottomPadding + layout.touchTarget > (drawerHeight ?? height);
  const drawerWidth = Math.min(340, Math.max(48, width - insets.right - 48));
  const brandScale = Math.min(1, Math.max(0, drawerWidth - insets.left - 40 - 48 - 8) / 149);
  return <View style={styles.overlay} accessibilityViewIsModal onAccessibilityEscape={onClose}>
    <Pressable onPress={onClose} accessibilityRole="button" accessibilityLabel="Close menu, outside area"
      style={styles.scrim} />
    <View onLayout={event => setDrawerHeight(event.nativeEvent.layout.height)} style={[styles.drawer, { width: drawerWidth }]}>
      <ScrollView testID="drawer-shell" style={styles.shell} scrollEnabled={wholeDrawerScroll}
        showsVerticalScrollIndicator={false} contentInsetAdjustmentBehavior="never"
        automaticallyAdjustContentInsets={false} contentContainerStyle={[styles.shellContent, !wholeDrawerScroll && styles.fixedContent, {
          paddingTop: topPadding, paddingBottom: bottomPadding, paddingLeft: insets.left + 20,
        }]}>
        <View testID="drawer-header" onLayout={event => setHeaderHeight(event.nativeEvent.layout.height)} style={styles.header}>
          <View accessible accessibilityRole="image" accessibilityLabel="GovAI" style={[styles.brand, { width: 149 * brandScale, height: 48 * brandScale }]}>
            <Image source={chatHomeMenuAssets.emblem} accessible={false} resizeMode="contain" style={{ width: 57 * brandScale, height: 48 * brandScale }} />
            <View style={{ width: 12 * brandScale }} />
            <Image source={chatHomeMenuAssets.wordmark} accessible={false} resizeMode="contain" style={{ width: 80 * brandScale, height: 26 * brandScale }} />
          </View>
          <Pressable ref={closeRef} onLayout={onCloseLayout} onPress={onClose} accessibilityRole="button" accessibilityLabel="Close menu" style={styles.closeTarget}>
            <View style={styles.closeDisc}><Image source={chatHomeMenuAssets.close} accessible={false} resizeMode="contain" style={styles.closeIcon} /></View>
          </Pressable>
        </View>
        <ScrollView testID="drawer-middle" style={wholeDrawerScroll ? styles.naturalMiddle : styles.middle}
          scrollEnabled={!wholeDrawerScroll} showsVerticalScrollIndicator={false}
          contentInsetAdjustmentBehavior="never" automaticallyAdjustContentInsets={false} contentContainerStyle={styles.content}>
        {[
          { label: 'New chat', asset: chatHomeMenuAssets.newChat, active: true },
          { label: 'Recent chats', asset: chatHomeMenuAssets.recent, active: false },
          { label: 'My applications', asset: chatHomeMenuAssets.applications, active: false },
        ].map(item => <Pressable key={item.label} disabled={!(item.active && onNewChat)} onPress={item.active ? onNewChat : undefined}
          accessibilityRole="button" accessibilityState={{ disabled: !(item.active && onNewChat) }}
          accessibilityLabel={item.active && onNewChat ? 'New chat' : `${item.label}. Unavailable in this development preview.`} style={[styles.primaryRow, item.active && styles.highlight]}>
          <Image source={item.asset} accessible={false} resizeMode="contain" style={styles.primaryIcon} />
          <Text style={styles.primaryText}>{item.label}</Text>
          {!item.active && <Image source={chatHomeMenuAssets.chevron} accessible={false} resizeMode="contain" style={styles.chevron} />}
        </Pressable>)}
        <View style={styles.divider} />
        <Text accessibilityRole="header" style={styles.sampleNotice}>Sample chat history · Actions unavailable</Text>
        <View style={styles.history}>
          {groups.map(group => <View key={group.label} style={styles.group}>
            <Text accessibilityRole="header" style={styles.groupLabel}>{group.label}</Text>
            {group.titles.map(title => <Pressable key={title} disabled accessibilityRole="button" accessibilityState={{ disabled: true }}
              accessibilityLabel={`${title}. Sample chat; opening and more actions are unavailable.`} style={styles.historyRow}>
              <Image source={chatHomeMenuAssets.history} accessible={false} resizeMode="contain" style={styles.historyIcon} />
              <Text style={styles.historyText}>{title}</Text>
              <Image source={chatHomeMenuAssets.more} accessible={false} resizeMode="contain" style={styles.historyIcon} />
            </Pressable>)}
          </View>)}
        </View>
        </ScrollView>
        <View testID="drawer-footer" onLayout={event => setFooterHeight(event.nativeEvent.layout.height)} style={styles.footer}>
        <Pressable disabled accessibilityRole="button" accessibilityState={{ disabled: true }}
          accessibilityLabel="Citizen. Development preview. Profile and settings are unavailable." style={styles.profile}>
          <View style={styles.profileAvatar}><Image source={chatHomeMenuAssets.profile} accessible={false} resizeMode="contain" style={styles.settings} /></View>
          <View style={styles.profileText}><Text style={styles.profileName}>Citizen</Text><Text style={styles.profileCaption}>Development preview</Text></View>
          <Image source={chatHomeMenuAssets.settings} accessible={false} resizeMode="contain" style={styles.settings} />
        </Pressable>
        <Pressable onPress={onLogout} disabled={!onLogout} accessibilityRole="button"
          accessibilityLabel="Logout" accessibilityState={{ disabled: !onLogout }}
          accessibilityHint={authenticated ? 'Ends your secure session and returns to Login.' : onLogout ? 'UI demo only. Returns to Login.' : 'Unavailable: no login occurred in this preview.'}
          style={styles.logout}>
          <Text style={styles.logoutText}>Logout</Text>
        </Pressable>
        <Text style={styles.sampleNotice}>{authenticated ? 'Ends your secure session.' : onLogout ? 'UI demo only · Returns to Login.' : 'Unavailable: no login occurred in this preview.'}</Text>
        </View>
      </ScrollView>
    </View>
  </View>;
}
const styles = StyleSheet.create({
  overlay: { position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, zIndex: 10 },
  scrim: { position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.55)', minWidth: layout.touchTarget, minHeight: layout.touchTarget },
  drawer: { position: 'absolute', left: 0, top: 0, bottom: 0, backgroundColor: '#fff', borderTopRightRadius: 24, borderBottomRightRadius: 24,
    overflow: 'hidden', boxShadow: '0px 10px 22px rgba(0,0,0,0.1)' },
  shell: { flex: 1 }, shellContent: { flexGrow: 1, paddingRight: 20 }, fixedContent: { height: '100%' },
  middle: { flex: 1, minHeight: 0 }, naturalMiddle: { flexGrow: 0, flexShrink: 0 },
  content: { gap: 4, paddingBottom: 4 }, footer: { flexShrink: 0, gap: 4 },
  header: { flexShrink: 0, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingBottom: 8, gap: 8 },
  brand: { flexDirection: 'row', alignItems: 'center', flexShrink: 0 },
  closeTarget: { width: layout.touchTarget, minHeight: layout.touchTarget, alignItems: 'center', justifyContent: 'center' },
  closeDisc: { width: 40, height: 40, borderRadius: 20, backgroundColor: '#f5f5f5', alignItems: 'center', justifyContent: 'center', boxShadow: '0px 1px 4px rgba(0,0,0,0.04)' },
  closeIcon: { width: 20, height: 20 },
  primaryRow: { minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 16, paddingHorizontal: 16, paddingVertical: 10, borderRadius: 16 },
  highlight: { backgroundColor: '#fcf3f5' }, primaryIcon: { width: 26, height: 26, flexShrink: 0 },
  primaryText: { flex: 1, fontFamily: 'Inter_600SemiBold', fontSize: 16, lineHeight: 22, color: '#142033' }, chevron: { width: 16, height: 16, flexShrink: 0 },
  divider: { height: 1, backgroundColor: '#e5e7eb' },
  sampleNotice: { paddingHorizontal: 12, paddingTop: 4, fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 17, color: '#666' },
  history: { gap: 8 }, group: { gap: 2 },
  groupLabel: { paddingHorizontal: 12, paddingVertical: 4, fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#666' },
  historyRow: { minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 8, gap: 14, borderRadius: 12 },
  historyIcon: { width: 20, height: 20, flexShrink: 0 }, historyText: { flex: 1, fontFamily: 'Inter_400Regular', fontSize: 15, lineHeight: 21, color: '#142033' },
  profile: { minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 12, paddingTop: 8, paddingHorizontal: 4, borderTopWidth: 1, borderTopColor: '#f1f5f9' },
  profileAvatar: { width: 40, height: 40, borderRadius: 20, alignItems: 'center', justifyContent: 'center', backgroundColor: '#fcf3f5' },
  profileText: { flex: 1, gap: 2 },
  logout: { minHeight: layout.touchTarget, minWidth: layout.touchTarget, borderWidth: 1, borderColor: '#a30f2d', borderRadius: 12, paddingHorizontal: 16, paddingVertical: 12, alignItems: 'center', justifyContent: 'center' },
  logoutText: { fontFamily: 'Inter_600SemiBold', fontSize: 16, lineHeight: 22, color: '#a30f2d' },
  profileName: { fontFamily: 'Inter_700Bold', fontSize: 14, lineHeight: 20, color: '#142033' }, profileCaption: { fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 17, color: '#666' },
  settings: { width: 22, height: 22, flexShrink: 0 },
});
