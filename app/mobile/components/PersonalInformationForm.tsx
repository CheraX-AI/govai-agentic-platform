import { StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import type { CitizenProfileController } from '../auth/useCitizenProfile';
import { ProfileField } from './ProfileField';
import { ProfileGenderField } from './ProfileGenderField';

export function PersonalInformationForm({ model, onFieldFocus }: { model: CitizenProfileController; onFieldFocus: (target: number) => void; }) {
  const { profile, draft, busy, change, changeAddress } = model;
  const { width, fontScale } = useWindowDimensions();
  const stack = width < 360 || fontScale > 1.2;
  if (!profile || !draft) return null;
  const common = { editable: !busy, onFieldFocus };
  return <View style={styles.groups}>
    <View style={styles.card}>
      <ProfileField label="Full name (English)" value={profile.full_name} readOnly hint="Registered name" />
      <ProfileField {...common} label="Full name (Sinhala)" value={draft.name_si} onChangeText={value => change('name_si', value)} script="si" />
      <ProfileField {...common} label="Full name (Tamil)" value={draft.name_ta} onChangeText={value => change('name_ta', value)} script="ta" />
    </View>
    <View style={[styles.card, styles.pair, stack && styles.stacked]}>
      <View style={styles.date}><ProfileField {...common} label="Date of birth" icon={require('../assets/figma/onboarding-track/calendar.png')} value={draft.date_of_birth} onChangeText={value => change('date_of_birth', value)} keyboardType="numbers-and-punctuation" placeholder="YYYY-MM-DD" hint="Year-month-day" autoCapitalize="none" /></View>
      <ProfileGenderField value={draft.gender} disabled={busy} onChange={value => change('gender', value)} />
    </View>
    <View style={styles.card}>
      <ProfileField label="Mobile number" value={profile.mobile_number} readOnly />
      <ProfileField label="Email address" value={profile.email ?? ''} placeholder="Not available" readOnly hint="Managed through your account settings" />
      <ProfileField label="NIC number" value={profile.nic} readOnly />
      <Text style={styles.hint}>Your registered name, NIC and mobile number cannot be changed here.</Text>
    </View>
    <View style={styles.card}>
      <Text accessibilityRole="header" style={styles.heading}>Address</Text>
      <ProfileField {...common} label="Address line 1" value={draft.address.line1} onChangeText={value => changeAddress('line1', value)} autoComplete="address-line1" />
      <ProfileField {...common} label="Address line 2 (optional)" value={draft.address.line2} onChangeText={value => changeAddress('line2', value)} autoComplete="address-line2" />
      <ProfileField {...common} label="City" value={draft.address.city} onChangeText={value => changeAddress('city', value)} autoComplete="postal-address-locality" />
    </View>
  </View>;
}
const styles = StyleSheet.create({
  groups: { gap: 14 }, stacked: { flexDirection: 'column', alignItems: 'stretch' }, card: { padding: 16, borderRadius: 18, backgroundColor: '#fff', borderWidth: 1, borderColor: '#edf0f4', gap: 16 },
  pair: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 12 }, date: { flex: 1, minWidth: 140 },
  hint: { fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 18, color: '#636d7c' }, heading: { fontFamily: 'Inter_600SemiBold', fontSize: 16, lineHeight: 23, color: '#142033' },
});
