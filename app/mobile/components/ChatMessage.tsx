import { Image, Pressable, StyleSheet, Text, View } from 'react-native';
import { conversationAssets, type ChatMessage as Message } from '../chatDemo';
import { layout } from '../layout';
import { OfficialSourceReference } from './OfficialSourceReference';
export function CitizenMessage({ message }: { message: Message }) {
  return <View style={styles.citizen}><View style={styles.bubble}><Text style={styles.citizenText}>{message.text}</Text></View><Text style={styles.caption}>{message.live ? 'You' : 'Local demo'}</Text></View>;
}
export function AiResponse({ message }: { message: Message }) {
  return <View style={styles.answer}>
    <View style={styles.from}><View style={styles.avatar}><Image source={conversationAssets.avatar} accessible={false} resizeMode="stretch" style={styles.avatarImage} /></View>
      <Text style={styles.name}>GovAI</Text><Text style={styles.caption}>{message.live ? 'AI response' : 'Local demo \u00b7 Mock response'}</Text></View>
    <Text style={styles.body}>{message.text}</Text>
    {message.sources?.map(source => <Text key={source.number} style={styles.caption} selectable>
      {`[${source.number}] ${source.title}${source.page ? ` - Page ${source.page}` : ''}${source.url ? `\n${source.url}` : ''}`}
    </Text>)}
    {message.nicSample && <>
      <View>{[1,2,3,4,5].map(number => <View key={number} style={styles.step}>
        <View style={styles.rail}><View style={styles.number}><Text style={styles.numberText}>{number}</Text></View>{number < 5 && <View style={styles.line} />}</View>
        <Text style={styles.stepText}>{`Sample step ${number} — demonstration only.`}</Text>
      </View>)}</View>
      <View style={styles.card}>
        <Text style={styles.caption}>Mock service details · Unverified</Text>
        <View style={styles.service}><View style={styles.iconBg}><Image source={conversationAssets.idCard} accessible={false} resizeMode="contain" style={{ width: 22, height: 22 }} /></View>
          <View style={{ flex: 1, gap: 2 }}><Text style={styles.cardTitle}>NIC renewal · Sample</Text><Text style={styles.caption}>Department for Registration of Persons</Text></View></View>
        <View style={styles.facts}>{['Fee','Time','Office visit'].map(label => <View key={label} style={styles.fact}><Text style={styles.caption}>{label}</Text><Text style={styles.factValue}>Not verified</Text></View>)}</View>
        <Pressable disabled accessibilityRole="button" accessibilityState={{ disabled: true }} accessibilityLabel="Start renewal. Unavailable in this local demo." style={styles.serviceAction}>
          <Text style={styles.actionText}>Start renewal</Text><Image source={conversationAssets.arrow} accessible={false} resizeMode="contain" style={{ width: 16, height: 16 }} />
        </Pressable>
      </View>
      <OfficialSourceReference icon={conversationAssets.globe} />
      <Text style={styles.name}>Ask next</Text>
      <View style={styles.chips}>{['Required documents','Find the nearest DRP office','Track my application'].map(label => <Pressable key={label} disabled accessibilityRole="button" accessibilityState={{ disabled: true }} accessibilityLabel={`${label}. Unavailable in this local demo.`} style={styles.chip}><Text style={styles.factValue}>{label}</Text></Pressable>)}</View>
    </>}
  </View>;
}
const styles=StyleSheet.create({
  citizen: { alignItems: 'flex-end', gap: 4 }, bubble: { maxWidth: '100%', paddingHorizontal: 16, paddingVertical: 12, backgroundColor: '#a30f2d', borderRadius: 20, borderBottomRightRadius: 2 },
  citizenText: { fontFamily: 'Inter_400Regular', fontSize: 15, lineHeight: 22, color: '#fff' }, caption: { fontFamily: 'Inter_400Regular', fontSize: 11, lineHeight: 17, color: '#666' },
  answer: { gap: 14 }, from: { flexDirection: 'row', alignItems: 'center', flexWrap: 'wrap', gap: 8 },
  avatar: { width: 32, height: 32, borderRadius: 16, borderWidth: 1, borderColor: '#e5e7eb', overflow: 'hidden' },
  // Original conversation crop (679:2416), distinct from the landing avatar crop.
  avatarImage: { position: 'absolute', width: 32 * .9974, height: 32 * .9974, left: 32 * .0125, top: 32 * .0075 },
  name: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 19, color: '#142033' }, body: { fontFamily: 'Inter_400Regular', fontSize: 15, lineHeight: 22, color: '#142033' },
  step: { flexDirection: 'row', gap: 12 }, rail: { width: 24, alignItems: 'center' }, number: { minWidth: 24, minHeight: 24, padding: 2, borderRadius: 999, borderWidth: 1, borderColor: '#fecdd3', backgroundColor: '#fcf3f5', alignItems: 'center', justifyContent: 'center' },
  numberText: { fontFamily: 'Inter_600SemiBold', fontSize: 12, lineHeight: 18, color: '#a30f2d' }, line: { width: 2, flex: 1, minHeight: 12, backgroundColor: '#fecdd3' },
  stepText: { flex: 1, paddingTop: 2, paddingBottom: 12, fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 21, color: '#444' },
  card: { padding: 16, gap: 14, borderWidth: 1, borderColor: '#e5e7eb', borderRadius: 16, boxShadow: '0px 2px 8px rgba(0,0,0,.03)' }, service: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  iconBg: { width: 40, height: 40, borderRadius: 14, backgroundColor: '#fcf3f5', alignItems: 'center', justifyContent: 'center' }, cardTitle: { fontFamily: 'Inter_700Bold', fontSize: 14, lineHeight: 21, color: '#142033' },
  facts: { flexDirection: 'row', flexWrap: 'wrap', padding: 12, gap: 12, backgroundColor: '#f8fafc', borderRadius: 12 }, fact: { flexGrow: 1, flexBasis: 80, gap: 2 },
  factValue: { fontFamily: 'Inter_600SemiBold', fontSize: 14, lineHeight: 21, color: '#142033' }, serviceAction: { minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 10, padding: 16, borderRadius: 14, backgroundColor: '#8b1a2e' },
  actionText: { flexShrink: 1, fontFamily: 'Inter_600SemiBold', fontSize: 16, lineHeight: 23, color: '#fff' }, source: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  chips: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 }, chip: { minHeight: layout.touchTarget, maxWidth: '100%', justifyContent: 'center', paddingVertical: 8, paddingHorizontal: 16, borderWidth: 1, borderColor: '#e5e7eb', borderRadius: 999 },
});
