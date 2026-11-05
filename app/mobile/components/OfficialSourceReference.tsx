import { useEffect, useRef, useState } from 'react';
import { Image, Linking, Pressable, StyleSheet, Text, View } from 'react-native';
import { layout } from '../layout';

const OFFICIAL_URL = 'https://drp.gov.lk/';
const DISCLAIMER = 'Official website · Demo reference, not verification of this response';

export function OfficialSourceReference({ icon }: { icon: number }) {
  const mounted = useRef(true);
  const opening = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  async function openOfficialWebsite() {
    if (!mounted.current || opening.current) return;
    opening.current = true; setBusy(true); setError('');
    try {
      // Only this approved homepage is handed to the OS. No response/user data.
      await Linking.openURL(OFFICIAL_URL);
    } catch {
      if (mounted.current) setError('Couldn’t open the official website. Please try again.');
    } finally {
      if (mounted.current) { opening.current = false; setBusy(false); }
    }
  }

  return <View>
    <Pressable onPress={openOfficialWebsite} disabled={busy} accessibilityRole="link"
      accessibilityLabel={`Department for Registration of Persons. ${DISCLAIMER}`}
      accessibilityHint="Opens in your external browser." accessibilityState={{ disabled: busy, busy }} style={styles.link}>
      <Image source={icon} accessible={false} resizeMode="contain" style={styles.icon} />
      <Text style={styles.label}>{DISCLAIMER}</Text>
    </Pressable>
    {!!error && <Text accessibilityRole="alert" accessibilityLiveRegion="polite" style={styles.error}>{error}</Text>}
  </View>;
}
const styles = StyleSheet.create({
  link: { minWidth: layout.touchTarget, minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 6, paddingVertical: 8 },
  icon: { width: 14, height: 14, flexShrink: 0 },
  label: { flex: 1, fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 18, color: '#a30f2d', textDecorationLine: 'underline' },
  error: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 19, color: '#a30f2d', paddingTop: 4 },
});
