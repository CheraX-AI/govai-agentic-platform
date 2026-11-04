import { StyleSheet } from 'react-native';

export const layout = { gutter: 24, contentWidth: 480, touchTarget: 48 };

// Font sizes stay independent of window width; native system scaling stays enabled.
export const typography = StyleSheet.create({
  heading: { fontSize: 30, lineHeight: 36 },
  body: { fontSize: 14, lineHeight: 20 },
  action: { fontSize: 16, lineHeight: 22 },
});
