import { Image, Pressable, StyleSheet, Text, View } from 'react-native';
import { conversationAssets } from '../chatDemo';
import { layout } from '../layout';
export function ChatBottomNavigation({
  assets,
  onHome,
}: {
  assets: { home: number; services: number; requests: number; profile: number };
  onHome?: () => void;
}) {
  return (
    <View style={styles.nav}>
      {[
        { label: 'Home', asset: assets.home, size: 18 },
        { label: 'Services', asset: assets.services, size: 22 },
        { label: 'My Requests', asset: assets.requests, size: 16.5 },
        { label: 'Profile', asset: onHome ? conversationAssets.profile : assets.profile, size: 22 },
      ].map((tab, index) => (
        <Pressable
          key={tab.label}
          disabled={!(index === 0 && onHome)}
          onPress={index === 0 ? onHome : undefined}
          accessibilityRole="tab"
          accessibilityState={{ disabled: !(index === 0 && onHome), selected: index === 0 }}
          accessibilityLabel={
            index === 0 && onHome
              ? 'Home. Return to Chat Home.'
              : `${tab.label}. Navigation unavailable in this preview.`
          }
          style={styles.tab}
        >
          <View style={[styles.navIcon, index === 0 && styles.subtle]}>
            <Image
              source={tab.asset}
              accessible={false}
              resizeMode="contain"
              style={{ width: tab.size, height: tab.size }}
            />
          </View>
          <Text style={[styles.tabLabel, index === 0 && styles.activeLabel]}>{tab.label}</Text>
        </Pressable>
      ))}
    </View>
  );
}
const styles = StyleSheet.create({
  nav: {
    flexDirection: 'row',
    alignItems: 'stretch',
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderTopWidth: 1,
    borderTopColor: '#e5e7eb',
  },
  tab: {
    flex: 1,
    minWidth: layout.touchTarget,
    minHeight: layout.touchTarget,
    alignItems: 'center',
    justifyContent: 'flex-start',
    gap: 2,
  },
  navIcon: {
    width: 36,
    height: 28,
    borderRadius: 9999,
    alignItems: 'center',
    justifyContent: 'center',
  },
  subtle: { backgroundColor: '#fcf3f5' },
  tabLabel: {
    width: '100%',
    fontFamily: 'Inter_500Medium',
    fontSize: 12,
    lineHeight: 17,
    color: '#666',
    textAlign: 'center',
  },
  activeLabel: { fontFamily: 'Inter_600SemiBold', color: '#a30f2d' },
});
