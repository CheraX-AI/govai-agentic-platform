import { useState } from 'react';
import { Image, Keyboard, Modal, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import type { ProfileFields } from '../services/profileApi';
import { layout } from '../layout';

export function ProfileGenderField({ value, disabled, onChange }: { value: ProfileFields['gender']; disabled: boolean; onChange: (value: ProfileFields['gender']) => void; }) {
  const [open, setOpen] = useState(false);
  const insets = useSafeAreaInsets();
  function choose(gender: ProfileFields['gender']) { onChange(gender); setOpen(false); }
  return <View style={styles.field}>
    <Text style={styles.label}>Gender</Text>
    <Pressable accessibilityRole="button" accessibilityLabel={`Gender. ${value === 'male' ? 'Male' : value === 'female' ? 'Female' : 'Not specified'}`}
      accessibilityHint="Opens gender choices" accessibilityState={{ disabled, expanded: open }} disabled={disabled}
      onPress={() => { Keyboard.dismiss(); setOpen(true); }} style={styles.select}>
      <Image source={require('../assets/figma/register/user.png')} accessible={false} resizeMode="contain" style={{ width: 16, height: 16 }} /><Text style={styles.value}>{value === 'male' ? 'Male' : value === 'female' ? 'Female' : 'Select gender'}</Text><View style={styles.chevron} />
    </Pressable>
    <Modal visible={open} transparent animationType="fade" onRequestClose={() => setOpen(false)}>
      <View style={[styles.backdrop, { paddingTop: insets.top + 20, paddingBottom: insets.bottom + 20, paddingLeft: insets.left + 24, paddingRight: insets.right + 24 }]}>
        <View style={styles.dialog} accessibilityViewIsModal>
          <ScrollView contentContainerStyle={styles.choices} keyboardShouldPersistTaps="handled">
            <Text accessibilityRole="header" style={styles.heading}>Gender</Text>
            {(['male', 'female'] as const).map(gender => <Pressable key={gender} accessibilityRole="radio"
              accessibilityState={{ checked: value === gender }} onPress={() => choose(gender)} style={styles.choice}>
              <Text style={styles.value}>{gender === 'male' ? 'Male' : 'Female'}</Text><View style={[styles.radio, value === gender && styles.checked]} />
            </Pressable>)}
            <Pressable accessibilityRole="button" onPress={() => choose('')} style={styles.choice}><Text style={styles.value}>Clear selection</Text></Pressable>
            <Pressable accessibilityRole="button" onPress={() => setOpen(false)} style={styles.choice}><Text style={styles.close}>Cancel</Text></Pressable>
          </ScrollView>
        </View>
      </View>
    </Modal>
  </View>;
}
const styles = StyleSheet.create({
  field: { gap: 7, flex: 1, minWidth: 140 }, label: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 19, color: '#142033' },
  select: { minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 12, borderWidth: 1, borderColor: '#e0e4ec', borderRadius: 12, padding: 12 },
  value: { fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 22, color: '#142033', flexShrink: 1 },
  chevron: { marginLeft: 'auto', width: 7, height: 7, borderRightWidth: 1.5, borderBottomWidth: 1.5, borderColor: '#556070', transform: [{ rotate: '45deg' }] },
  backdrop: { flex: 1, backgroundColor: '#14203366', justifyContent: 'center', alignItems: 'center' },
  dialog: { width: '100%', maxWidth: 380, maxHeight: '100%', backgroundColor: '#fff', borderRadius: 20 }, choices: { padding: 20, gap: 8 },
  heading: { fontFamily: 'Inter_700Bold', fontSize: 18, lineHeight: 26, color: '#142033', marginBottom: 8 },
  choice: { minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', padding: 12, gap: 12 },
  radio: { width: 18, height: 18, borderRadius: 9, borderWidth: 1, borderColor: '#9ca3af' }, checked: { borderWidth: 5, borderColor: '#891b30' },
  close: { fontFamily: 'Inter_600SemiBold', color: '#891b30', fontSize: 14, lineHeight: 22 },
});
