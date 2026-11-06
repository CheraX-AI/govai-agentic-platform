import { StyleSheet, View } from 'react-native';

// Decorative position markers only; no gestures or focusable controls.

export function OnboardingProgress({ current }: { current: 1 | 2 | 3 | 4 | 5 }) {
  return (
    <View
      accessible={false}
      importantForAccessibility="no-hide-descendants"
      pointerEvents="none"
      style={styles.row}
    >
      {[1, 2, 3, 4, 5].map(step => (
        <View key={step} style={[styles.dot, step === current && styles.active]} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  dot: { width: 8, height: 8, borderRadius: 9999, backgroundColor: '#feecee' },
  active: { width: 24, backgroundColor: '#a30f2d' },
});
