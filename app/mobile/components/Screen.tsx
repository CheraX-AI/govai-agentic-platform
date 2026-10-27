import type { PropsWithChildren } from 'react';
import { ScrollView, StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { layout } from '../layout';

type Props = PropsWithChildren<{
  scroll?: boolean;
  horizontalPadding?: number;
  contentStyle?: StyleProp<ViewStyle>;
}>;

// Owns all content insets. Full-bleed backgrounds belong outside this component.
// Forms should choose keyboard handling when implemented, not inherit it blindly.
export function Screen({ children, scroll = true, horizontalPadding = layout.gutter, contentStyle }: Props) {
  const insets = useSafeAreaInsets();
  const padding = {
    paddingTop: insets.top + 22,
    paddingBottom: insets.bottom + 10,
    paddingLeft: insets.left + horizontalPadding,
    paddingRight: insets.right + horizontalPadding,
  };
  const content = <View style={[styles.content, contentStyle]}>{children}</View>;
  return scroll ? (
    <ScrollView style={styles.root} contentContainerStyle={[styles.container, padding]}>
      {content}
    </ScrollView>
  ) : <View style={[styles.root, padding]}>{content}</View>;
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  container: { flexGrow: 1 },
  content: { flexGrow: 1, width: '100%', maxWidth: layout.contentWidth, alignSelf: 'center' },
});
