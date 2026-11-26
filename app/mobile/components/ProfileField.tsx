import { Image, StyleSheet, Text, TextInput, View } from 'react-native';
import type { TextInputProps } from 'react-native';
import { layout } from '../layout';

type Props = TextInputProps & { label: string; hint?: string; readOnly?: boolean; script?: 'si' | 'ta'; icon?: number; onFieldFocus?: (target: number) => void; };
export function ProfileField({ label, hint, readOnly, script, icon, onFieldFocus, ...input }: Props) {
  return <View style={styles.field}>
    <Text style={styles.label}>{label}</Text>
    <View style={[styles.inputRow, readOnly && styles.readonly]}>
      <Image source={icon ?? (readOnly ? require('../assets/figma/register/lock.png') : require('../assets/figma/register/user.png'))} accessible={false} resizeMode="contain" style={styles.icon} />
      <TextInput {...input} editable={!readOnly && input.editable !== false}
        accessibilityLabel={`${label}${readOnly ? '. Read only' : ''}`}
        accessibilityHint={hint} onFocus={event => onFieldFocus?.(event.nativeEvent.target)}
        placeholderTextColor="#758094" autoCorrect={false}
        style={[styles.input, script === 'si' && styles.sinhala, script === 'ta' && styles.tamil, input.style]} />
    </View>
    {!!hint && <Text style={styles.hint}>{hint}</Text>}
  </View>;
}
const styles = StyleSheet.create({
  field: { gap: 7, minWidth: 0 },
  label: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 19, color: '#142033' },
  inputRow: { borderWidth: 1, borderColor: '#e0e4ec', borderRadius: 12, minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 12, backgroundColor: '#fff' },
  readonly: { backgroundColor: '#fafbfc' },
  icon: { width: 16, height: 16 },
  input: { flex: 1, minWidth: 0, paddingVertical: 12, color: '#142033', fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 22 },
  sinhala: { fontFamily: 'Yaldevi_400Regular' }, tamil: { fontFamily: 'NotoSansTamil_400Regular' },
  hint: { fontFamily: 'Inter_400Regular', color: '#636d7c', fontSize: 12, lineHeight: 18 },
});
