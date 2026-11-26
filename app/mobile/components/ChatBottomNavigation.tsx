import { Image, Pressable, StyleSheet, Text, View } from 'react-native';
import { conversationAssets } from '../chatDemo';
import { layout } from '../layout';
export function ChatBottomNavigation({ assets, onHome, onProfile, selectedTab = 'home' }: { assets: { home: number; services: number; requests: number; profile: number; }; onHome?: () => void; onProfile?: () => void; selectedTab?: 'home' | 'profile'; }) {
  return <View style={styles.nav}>{[
    { label: 'Home', asset: assets.home, size: 18 }, { label: 'Services', asset: assets.services, size: 22 },
    { label: 'My Requests', asset: assets.requests, size: 16.5 }, { label: 'Profile', asset: onHome ? conversationAssets.profile : assets.profile, size: 22 },
  ].map((tab, index) => {
    const action = index === 0 ? onHome : index === 3 ? onProfile : undefined;
    const selected = index === (selectedTab === 'profile' ? 3 : 0);
    return <Pressable key={tab.label} disabled={!action} onPress={action}
      accessibilityRole="tab" accessibilityState={{ disabled: !action, selected }}
      accessibilityLabel={action ? tab.label : `${tab.label}. Navigation unavailable.`} style={styles.tab}>
      <View style={[styles.navIcon, selected && styles.subtle]}><Image source={tab.asset} accessible={false} resizeMode="contain" style={{ width: tab.size, height: tab.size, tintColor: selectedTab === 'profile' && selected ? '#891b30' : undefined }} /></View>
      <Text style={[styles.tabLabel, selected && (selectedTab === 'profile' ? styles.profileActiveLabel : styles.activeLabel)]}>{tab.label}</Text>
    </Pressable>;
  })}</View>;
}
const styles = StyleSheet.create({
  nav: { flexDirection: 'row', alignItems: 'stretch', paddingHorizontal: 16, paddingVertical: 8, borderTopWidth: 1, borderTopColor: '#e5e7eb' },
  tab: { flex: 1, minWidth: layout.touchTarget, minHeight: layout.touchTarget, alignItems: 'center', justifyContent: 'flex-start', gap: 2 },
  navIcon: { width: 36, height: 28, borderRadius: 9999, alignItems: 'center', justifyContent: 'center' }, subtle: { backgroundColor: '#fcf3f5' },
  tabLabel: { width: '100%', fontFamily: 'Inter_500Medium', fontSize: 12, lineHeight: 17, color: '#666', textAlign: 'center' }, activeLabel: { fontFamily: 'Inter_600SemiBold', color: '#a30f2d' }, profileActiveLabel: { fontFamily: 'Inter_600SemiBold', color: '#891b30' },
});
