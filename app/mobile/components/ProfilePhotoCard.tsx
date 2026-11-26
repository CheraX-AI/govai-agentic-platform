import { Image, Pressable, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import type { CitizenProfileController } from '../auth/useCitizenProfile';
import { layout } from '../layout';

export function ProfilePhotoCard({ model }: { model: CitizenProfileController; }) {
  const { fontScale } = useWindowDimensions();
  const avatarSize = Math.max(76, fontScale * 34 + 24);
  const blocked = model.busy || model.needsReload;
  return <View style={styles.card}>
    <View style={styles.row}>
      <View style={[styles.avatar, { width: avatarSize, height: avatarSize, borderRadius: avatarSize / 2 }]}>{model.imageUri ? <Image source={{ uri: model.imageUri }} accessibilityLabel="Your profile photo" onError={model.reportPhotoError} style={styles.image} /> : <Text style={styles.initials}>{model.initials}</Text>}</View>
      <View style={styles.controls}>
        <Pressable accessibilityRole="button" accessibilityState={{ disabled: blocked }} disabled={blocked} onPress={() => void model.choosePhoto()} style={styles.button}><Text style={styles.action}>Change photo</Text></Pressable>
        <Text style={styles.hint}>JPG or PNG, up to 2 MiB</Text>
      </View>
    </View>
    {model.selectedPhoto && <View style={styles.actions}>
      <Text style={styles.hint}>Photo selected. Not yet saved.</Text>
      <Pressable accessibilityRole="button" accessibilityState={{ disabled: blocked }} disabled={blocked} onPress={() => void model.uploadPhoto()} style={[styles.button, styles.primary]}><Text style={styles.white}>Upload photo</Text></Pressable>
      <Pressable accessibilityRole="button" disabled={model.busy} accessibilityState={{ disabled: model.busy }} onPress={model.cancelPhoto} style={styles.button}><Text style={styles.action}>Cancel photo</Text></Pressable>
    </View>}
    {!!model.photoError && <View><Text accessibilityRole="alert" style={styles.error}>{model.photoError}</Text>
      {!!model.profile?.photo_url && !model.selectedPhoto && <Pressable accessibilityRole="button" disabled={model.busy} onPress={model.retryPhoto} style={styles.button}><Text style={styles.action}>Retry photo</Text></Pressable>}
    </View>}
  </View>;
}
const styles = StyleSheet.create({
  card: { backgroundColor: '#fff', borderRadius: 18, borderWidth: 1, borderColor: '#edf0f4', padding: 16, gap: 12 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 16, flexWrap: 'wrap' },
  avatar: { width: 76, height: 76, borderRadius: 38, backgroundColor: '#f6e0e5', alignItems: 'center', justifyContent: 'center', overflow: 'hidden' },
  initials: { fontFamily: 'Inter_700Bold', fontSize: 25, lineHeight: 34, color: '#891b30' }, image: { width: '100%', height: '100%', resizeMode: 'cover' },
  controls: { flex: 1, minWidth: 170, gap: 8 }, button: { minHeight: layout.touchTarget, borderRadius: 12, borderWidth: 1, borderColor: '#891b30', justifyContent: 'center', alignItems: 'center', padding: 12 },
  action: { fontFamily: 'Inter_600SemiBold', color: '#891b30', fontSize: 14, lineHeight: 22, textAlign: 'center' },
  hint: { fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 18, color: '#636d7c' },
  actions: { gap: 10 }, primary: { backgroundColor: '#891b30' }, white: { fontFamily: 'Inter_600SemiBold', color: '#fff', fontSize: 14, lineHeight: 22, textAlign: 'center' },
  error: { fontFamily: 'Inter_400Regular', color: '#891b30', fontSize: 13, lineHeight: 20, marginBottom: 8 },
});
