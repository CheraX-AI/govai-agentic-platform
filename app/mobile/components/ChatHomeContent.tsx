import type { ChatReply } from '../services/chatApi';
import { useEffect, useRef, useState } from 'react';
import { useFonts } from 'expo-font';
import { StatusBar } from 'expo-status-bar';
import { AccessibilityInfo, Animated, BackHandler, Image, Keyboard, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { layout } from '../layout';
import { ChatHomeMenu, chatHomeMenuAssets } from './ChatHomeMenu';
import { ChatHeader } from './ChatHeader';
import { ChatComposer } from './ChatComposer';
import { ChatBottomNavigation } from './ChatBottomNavigation';
import { ChatConversation, type ChatFollowController } from './ChatConversation';
import { conversationAssets, localAnswer, type LocalThread } from '../chatDemo';

export const chatHomeAssets = {
  avatar: require('../assets/figma/chat-home/avatar.png'),
  menu: require('../assets/figma/chat-home/menu.png'),
  notifications: require('../assets/figma/chat-home/notifications.png'),
  message: require('../assets/figma/chat-home/message.png'),
  plus: require('../assets/figma/chat-home/plus.png'),
  mic: require('../assets/figma/chat-home/mic.png'),
  send: require('../assets/figma/chat-home/send.png'),
  home: require('../assets/figma/chat-home/home.png'),
  services: require('../assets/figma/chat-home/services.png'),
  requests: require('../assets/figma/chat-home/requests.png'),
  profile: require('../assets/figma/chat-home/profile.png'),
};
const preloadAssets = { ...chatHomeAssets, ...Object.fromEntries(Object.entries(chatHomeMenuAssets).map(([key, source]) => ['drawer-' + key, source])), ...Object.fromEntries(Object.entries(conversationAssets).map(([key, source]) => ['conversation-' + key, source])) };
const suggestions = ['How do I renew my NIC?', 'What do I need for a passport?', 'Pay my vehicle licence'];


export type ChatHomeProps = { onExit: () => void; mode: 'standalone' | 'demo' | 'authenticated'; authenticatedDisplayName?: string | null; onSendChat?: (message: string, signal: AbortSignal) => Promise<ChatReply> };

// Only a future backend-authenticated adapter should supply this name.
export function ChatHomeContent(props: ChatHomeProps) {
  if (props.mode !== 'authenticated' && (!__DEV__ || (props.mode === 'demo' ? process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO !== '1' : process.env.EXPO_PUBLIC_CHAT_HOME_PREVIEW !== '1'))) return null;
  return <ChatHomeBody {...props} />;
}

function ChatHomeBody({ onExit, mode, authenticatedDisplayName, onSendChat }: ChatHomeProps) {
  const name = authenticatedDisplayName?.trim();
  const greeting = name ? `Hello, ${name}` : 'Hello';
  const insets = useSafeAreaInsets();
  const [fontsLoaded, fontError] = useFonts({ Fraunces_700Bold: require('../assets/figma/chat-home/fonts/Fraunces-Bold.ttf') });
  const [loaded, setLoaded] = useState<Set<string>>(() => new Set());
  const [assetError, setAssetError] = useState(false);
  const [chromeHeights, setChromeHeights] = useState<Record<string, number>>({});
  const [draft, setDraft] = useState('');
  const [conversationDraft, setConversationDraft] = useState('');
  const [thread, setThread] = useState<LocalThread | null>(null);
  const threadRef = useRef<LocalThread | null>(null);
  const follow = useRef<ChatFollowController>({ threadId: null, revision: 0 });
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);
  const [messageError, setMessageError] = useState('');
  const [failNext, setFailNext] = useState(false);
  const failNextRef = useRef(false);
  const sequence = useRef(0);
  const responseAbort = useRef<AbortController | null>(null);
  const responseTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const request = useRef<{ thread: number; prompt: string; status: 'pending' | 'failed' } | null>(null);
  const [menuVisible, setMenuVisible] = useState(false);
  const [reduceMotion, setReduceMotion] = useState(true);
  const menuPhase = useRef<'closed' | 'open' | 'closing'>('closed');
  const menuButton = useRef<View>(null);
  const closeButton = useRef<View>(null);
  const menuOpacity = useRef(new Animated.Value(0)).current;
  const focusFrame = useRef<number | null>(null);
  const hadMenu = useRef(false);
  const [overlap, setOverlap] = useState(0);
  const input = useRef<TextInput>(null);
  const viewport = useRef<View>(null);
  const composer = useRef<View>(null);
  const scroll = useRef<ScrollView>(null);
  const focused = useRef(false);
  const keyboardTop = useRef<number | null>(null);
  const offset = useRef(0);
  const frame = useRef<number | null>(null);
  const leaving = useRef(false);
  const ready = fontsLoaded && loaded.size === Object.keys(preloadAssets).length && !assetError;
  const actionVersion = sequence.current;
  const retryAttempt = request.current;
  function measureChrome(key: string, height: number) {
    setChromeHeights(previous => Math.abs((previous[key] ?? 0) - height) < .5 ? previous : { ...previous, [key]: height });
  }

  function cancelResponse() {
    responseAbort.current?.abort(); responseAbort.current = null;
    follow.current.threadId=null; follow.current.revision++;
    if (responseTimer.current !== null) clearTimeout(responseTimer.current);
    responseTimer.current = null; request.current = null; setPending(false); setFailed(false);
  }
  function captureFollow() { follow.current.capture?.(); follow.current.revision++; }
  function prepareResponse(current: LocalThread, prompt: string) {
    captureFollow();
    const attempt = { thread: current.id, prompt, status: 'pending' as 'pending' | 'failed' };
    request.current = attempt; setPending(true); setFailed(false);
    if (mode === 'authenticated') {
      const controller = new AbortController(); responseAbort.current = controller;
      void (async () => {
        try {
          if (!onSendChat) throw new Error('Chat configuration is unavailable.');
          const reply = await onSendChat(prompt, controller.signal);
          if (controller.signal.aborted || leaving.current || request.current !== attempt || threadRef.current?.id !== attempt.thread) return;
          captureFollow();
          const updated = { ...threadRef.current, messages: [...threadRef.current.messages, { id: ++sequence.current, role: 'ai' as const, text: reply.answer, sources: reply.sources, live: true }] };
          threadRef.current = updated; setThread(updated); request.current = null; setPending(false); setMessageError('');
        } catch (error) {
          if (controller.signal.aborted || leaving.current || request.current !== attempt) return;
          attempt.status = 'failed'; setPending(false); setFailed(true);
          setMessageError(error instanceof Error ? error.message : 'Chat is unavailable. Please try again later.');
        } finally { if (responseAbort.current === controller) responseAbort.current = null; }
      })();
      return;
    }
    const shouldFail = failNextRef.current; failNextRef.current = false; setFailNext(false);
    responseTimer.current = setTimeout(() => {
      if (leaving.current || request.current !== attempt || threadRef.current?.id !== attempt.thread) return;
      captureFollow();
      responseTimer.current = null;
      setPending(false);
      if (shouldFail) { attempt.status = 'failed'; setFailed(true); return; }
      const updated = { ...threadRef.current, messages: [...threadRef.current.messages, { id: ++sequence.current, role: 'ai' as const, ...localAnswer(prompt) }] };
      threadRef.current = updated; setThread(updated); request.current = null;
    }, 800);
  }
  function startConversation(prompt: string, submittedDraft = false) {
    if (!ready || leaving.current || menuPhase.current !== 'closed' || threadRef.current || sequence.current !== actionVersion) return;
    if ([...prompt.trim()].length > 4000) { setMessageError('Enter a question of at most 4000 characters.'); return; }
    if (!prompt.trim()) { setMessageError('Enter a message.'); input.current?.focus(); return; }
    const current: LocalThread = { id: ++sequence.current, title: prompt.trim() === suggestions[0] ? 'Renew your NIC' : 'Sample conversation',
      messages: [{ id: ++sequence.current, role: 'citizen', live: mode === 'authenticated', text: prompt.trim() }] };
    threadRef.current = current; setThread(current); setConversationDraft(''); setMessageError('');
    follow.current.threadId=current.id;
    if (submittedDraft) { setDraft(''); input.current?.clear(); }
    input.current?.blur(); focused.current = false; Keyboard.dismiss(); offset.current = 0; scroll.current?.scrollTo({ y: 0, animated: false });
    prepareResponse(current, prompt.trim());
  }
  function sendMessage() {
    if (leaving.current || menuPhase.current !== 'closed' || request.current || threadRef.current?.id !== thread?.id || sequence.current !== actionVersion) return;
    const current = threadRef.current;
    if (!current) { startConversation(draft, true); return; }
    if (!conversationDraft.trim()) { setMessageError('Enter a message.'); input.current?.focus(); return; }
    const prompt = conversationDraft.trim();
    if ([...prompt].length > 4000) { setMessageError('Enter a question of at most 4000 characters.'); return; }
    captureFollow();
    const updated = { ...current, messages: [...current.messages, { id: ++sequence.current, role: 'citizen' as const, live: mode === 'authenticated', text: prompt }] };
    threadRef.current = updated; setThread(updated); setConversationDraft(''); input.current?.clear(); setMessageError('');
    prepareResponse(updated, prompt);
  }
  function retryResponse() {
    if (leaving.current || request.current !== retryAttempt || request.current?.status !== 'failed' || !threadRef.current || threadRef.current.id !== thread?.id) return;
    prepareResponse(threadRef.current, request.current.prompt);
  }
  function returnToLanding(clearLanding = false) {
    if (leaving.current) return;
    sequence.current++;
    cancelResponse(); threadRef.current = null; setThread(null); setConversationDraft(''); setMessageError('');
    failNextRef.current = false; setFailNext(false); input.current?.clear(); input.current?.blur();
    if (clearLanding) setDraft('');
    focused.current = false; keyboardTop.current = null; offset.current = 0; setOverlap(0); Keyboard.dismiss();
    scroll.current?.scrollTo({ y: 0, animated: false });
    if (frame.current !== null) cancelAnimationFrame(frame.current); frame.current = null;
  }
  function newChat() { if(sequence.current!==actionVersion || threadRef.current?.id!==thread?.id)return; returnToLanding(true); if (menuPhase.current === 'open') closeMenu(); }

  function focusControl(target: View | null) {
    if (!target) return;
    target.focus(); AccessibilityInfo.sendAccessibilityEvent(target, 'focus');
  }
  function focusClose() {
    if (leaving.current) return;
    if (focusFrame.current !== null) cancelAnimationFrame(focusFrame.current);
    focusFrame.current = requestAnimationFrame(() => {
      focusFrame.current = null;
      if (!leaving.current && menuPhase.current === 'open') focusControl(closeButton.current);
    });
  }
  function openMenu() {
    if (!ready || leaving.current || menuPhase.current !== 'closed') return;
    if (focusFrame.current !== null) cancelAnimationFrame(focusFrame.current);
    focusFrame.current = null; hadMenu.current = true; menuPhase.current = 'open';
    input.current?.blur(); focused.current = false; Keyboard.dismiss();
    menuOpacity.setValue(0); setMenuVisible(true);
  }
  function closeMenu() {
    if (leaving.current || menuPhase.current !== 'open') return;
    menuPhase.current = 'closing'; menuOpacity.stopAnimation();
    Animated.timing(menuOpacity, { toValue: 0, duration: reduceMotion ? 0 : 250, useNativeDriver: true }).start(({ finished }) => {
      if (finished && menuPhase.current === 'closing') { menuPhase.current = 'closed'; setMenuVisible(false); }
    });
  }
  function logout() {
    if ((mode !== 'authenticated' && (!__DEV__ || mode !== 'demo' || process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO !== '1')) || leaving.current || menuPhase.current !== 'open') return;
    leaving.current = true; menuPhase.current = 'closed'; hadMenu.current = false;
    cancelResponse(); threadRef.current = null; setThread(null); setConversationDraft(''); setMessageError(''); failNextRef.current = false; setFailNext(false);
    menuOpacity.stopAnimation(); menuOpacity.setValue(0);
    if (focusFrame.current !== null) cancelAnimationFrame(focusFrame.current);
    if (frame.current !== null) cancelAnimationFrame(frame.current);
    focusFrame.current = null; frame.current = null;
    input.current?.clear(); input.current?.blur(); setDraft(''); focused.current = false;
    keyboardTop.current = null; offset.current = 0; setOverlap(0); setMenuVisible(false);
    Keyboard.dismiss(); onExit();
  }
  useEffect(() => {
    let mounted = true;
    AccessibilityInfo.isReduceMotionEnabled().then(value => { if (mounted) setReduceMotion(value); });
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    return () => { responseAbort.current?.abort(); mounted = false; leaving.current = true; follow.current.threadId=null; follow.current.revision++; if(responseTimer.current!==null)clearTimeout(responseTimer.current); request.current=null; threadRef.current=null; subscription.remove(); menuOpacity.stopAnimation(); if (focusFrame.current !== null) cancelAnimationFrame(focusFrame.current); };
  }, []);
  useEffect(() => {
    if (menuVisible && menuPhase.current === 'open') {
      Animated.timing(menuOpacity, { toValue: 1, duration: reduceMotion ? 0 : 200, useNativeDriver: true }).start();
    } else if (!menuVisible && hadMenu.current) {
      hadMenu.current = false;
      focusFrame.current = requestAnimationFrame(() => {
        focusFrame.current = null;
        if (menuPhase.current === 'closed' && !leaving.current) focusControl(menuButton.current);
      });
    }
  }, [menuVisible, reduceMotion]);

  function adjustVisibility() {
    const top = keyboardTop.current;
    if (top === null) return;
    viewport.current?.measureInWindow((_x, y, _w, height) => {
      if (keyboardTop.current !== top) return;
      const bottom = Math.min(y + height, top);
      // Overlay keyboards need only their actual overlap. Resized windows add zero.
      const next = Math.max(0, y + height - top);
      setOverlap(previous => Math.abs(previous - next) < 0.5 ? previous : next);
      // Only the outer shell moves to expose controls; the conversation's own history offset is untouched.
      if (!focused.current) return;
      composer.current?.measureInWindow((_cx, cy, _cw, ch) => {
        if (keyboardTop.current !== top || !focused.current) return;
        const delta = ch > bottom - y - 24 ? cy - y - 12
          : cy < y + 12 ? cy - y - 12 : cy + ch > bottom - 12 ? cy + ch - bottom + 12 : 0;
        if (Math.abs(delta) > 1) scroll.current?.scrollTo({ y: Math.max(0, offset.current + delta), animated: false });
      });
    });
  }
  function scheduleVisibility() {
    if (frame.current !== null) cancelAnimationFrame(frame.current);
    frame.current = requestAnimationFrame(() => {
      frame.current = requestAnimationFrame(() => { frame.current = null; adjustVisibility(); });
    });
  }
  useEffect(() => {
    const mountedInput = input.current;
    const show = Keyboard.addListener('keyboardDidShow', event => { keyboardTop.current = event.endCoordinates.screenY; scheduleVisibility(); });
    const change = Keyboard.addListener('keyboardDidChangeFrame', event => {
      if (Keyboard.isVisible() || keyboardTop.current !== null) { keyboardTop.current = event.endCoordinates.screenY; scheduleVisibility(); }
    });
    const hide = Keyboard.addListener('keyboardDidHide', () => {
      keyboardTop.current = null; setOverlap(0);
      if (frame.current !== null) cancelAnimationFrame(frame.current);
      frame.current = null;
    });
    return () => {
      show.remove(); change.remove(); hide.remove();
      if (frame.current !== null) cancelAnimationFrame(frame.current);
      mountedInput?.clear(); Keyboard.dismiss();
    };
  }, [ready]);
  useEffect(() => {
    const back = BackHandler.addEventListener('hardwareBackPress', () => {
      if (menuPhase.current !== 'closed') { closeMenu(); return true; }
      if (Keyboard.isVisible()) { Keyboard.dismiss(); return true; }
      if (threadRef.current) { returnToLanding(); return true; }
      if (!leaving.current) {
        leaving.current = true; input.current?.clear(); setDraft(''); focused.current = false; Keyboard.dismiss(); onExit();
      }
      return true;
    });
    return () => back.remove();
  }, [onExit, reduceMotion]);

  return <View style={styles.root}>
    <StatusBar style="dark" />
    <View pointerEvents={menuVisible ? 'none' : 'auto'} accessibilityElementsHidden={menuVisible}
      importantForAccessibility={menuVisible ? 'no-hide-descendants' : 'auto'} style={[styles.root, {
      paddingTop: insets.top, paddingBottom: insets.bottom, paddingLeft: insets.left, paddingRight: insets.right,
    }]}>
      <View ref={viewport} onLayout={scheduleVisibility} style={styles.root}>
      <ScrollView ref={scroll} testID="chat-shell" style={styles.root} contentContainerStyle={[styles.content, thread && { height: '100%', minHeight: Object.values(chromeHeights).reduce((sum, value) => sum + value, 0) + 2 * layout.touchTarget + overlap }, { paddingBottom: overlap }]}
        showsVerticalScrollIndicator={false} keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag"
        automaticallyAdjustKeyboardInsets={false} automaticallyAdjustContentInsets={false} contentInsetAdjustmentBehavior="never"
        onContentSizeChange={scheduleVisibility} onScroll={event => { offset.current = event.nativeEvent.contentOffset.y; }} scrollEventThrottle={16}>
        <View testID="chat-header" onLayout={event => measureChrome('header',event.nativeEvent.layout.height)}><ChatHeader title={thread?.title} onBack={() => { if(threadRef.current?.id===thread?.id)returnToLanding(); }} onNewChat={newChat} onMenu={openMenu} menuRef={menuButton}
          ready={ready} menuVisible={menuVisible} menuAsset={chatHomeAssets.menu} notificationAsset={chatHomeAssets.notifications} /></View>
        <View testID="chat-notices" onLayout={event => measureChrome('notices',event.nativeEvent.layout.height)}>
        <Text style={styles.notice}>{mode === 'authenticated' ? 'Ask GovAI. Check the cited sources before acting on guidance.' : mode === 'demo' ? 'UI demo · Credentials were not verified. AI and actions are disconnected.' : 'Development preview · Local mock chat works. Authentication, AI and backend are disconnected; service actions are unavailable.'}</Text>
        {!!thread && mode !== 'authenticated' && <Text style={styles.notice}>Local demo · Responses are samples, not verified government advice. AI is disconnected.</Text>}
        </View>
        {__DEV__ && mode !== 'authenticated' && ready && !!thread && <View testID="chat-demo-controls" onLayout={event => measureChrome('demo',event.nativeEvent.layout.height)} style={styles.demoControls}>
          <Pressable accessibilityRole="switch" accessibilityLabel="Simulate next response error" accessibilityState={{ checked: failNext }}
            onPress={() => { if(leaving.current)return; failNextRef.current=!failNextRef.current;setFailNext(failNextRef.current); }} style={styles.errorSwitch}>
            <Text style={styles.demoControlText}>{`Demo controls · Simulate next response error: ${failNext ? 'On' : 'Off'}`}</Text>
          </Pressable>
        </View>}
        {!ready ? <View style={styles.loading}>
          <Text accessibilityRole={fontError || assetError ? 'alert' : undefined} accessibilityLiveRegion="polite" style={styles.body}>
            {fontError || assetError ? 'Unable to load Chat Home preview assets. Reload the preview.' : 'Loading Chat Home preview…'}
          </Text>
        </View> : <>
          {thread ? <ChatConversation live={mode === 'authenticated'} key={thread.id} thread={thread} pending={pending} failed={failed} onRetry={retryResponse} follow={follow} /> : <View style={styles.landing}>
            <View style={styles.greeting}>
              <View style={styles.avatar} accessible accessibilityRole="image" accessibilityLabel="GovAI assistant illustration">
                <Image source={chatHomeAssets.avatar} accessible={false} resizeMode="stretch" style={styles.avatarImage} />
              </View>
              <View style={styles.greetingText}>
                <Text style={styles.hello} accessibilityLabel={greeting}>{greeting}</Text>
                <Text accessibilityRole="header" style={styles.heading}>How can I help you today?</Text>
              </View>
            </View>
            <View style={styles.suggestions}>
              {suggestions.map(text => <Pressable key={text} onPress={() => startConversation(text)} accessibilityRole="button" accessibilityState={{ disabled: false }}
                accessibilityLabel={text} style={styles.suggestion}>
                <Image source={chatHomeAssets.message} accessible={false} resizeMode="contain" style={styles.messageIcon} />
                <Text style={styles.suggestionText}>{text}</Text>
              </Pressable>)}
            </View>
          </View>
          }
          <View ref={composer} testID="chat-composer" onLayout={event => measureChrome('composer',event.nativeEvent.layout.height)}>
            <ChatComposer inputRef={input} value={thread ? conversationDraft : draft}
              onChange={text => { if(leaving.current)return; if(threadRef.current)setConversationDraft(text);else setDraft(text); setMessageError(''); }}
              onSend={sendMessage} blocked={menuVisible} pending={pending || failed} conversation={!!thread} error={messageError} assets={chatHomeAssets}
              onFocus={() => { focused.current = true; const metrics = Keyboard.metrics(); if(metrics)keyboardTop.current=metrics.screenY; scheduleVisibility(); }}
              onBlur={() => { focused.current=false; }} onLayout={scheduleVisibility} onContentSizeChange={scheduleVisibility} />
          </View>
          <View testID="chat-navigation" onLayout={event => measureChrome('navigation',event.nativeEvent.layout.height)}><ChatBottomNavigation assets={chatHomeAssets} onHome={thread ? () => { if(threadRef.current?.id===thread?.id)returnToLanding(); } : undefined} /></View>
        </>}
      </ScrollView>
      </View>
    </View>
    {menuVisible && <Animated.View style={[StyleSheet.absoluteFill, { opacity: menuOpacity, zIndex: 10 }]}>
      <ChatHomeMenu authenticated={mode === 'authenticated'} onClose={closeMenu} onCloseLayout={focusClose} closeRef={closeButton} onLogout={mode === 'demo' || mode === 'authenticated' ? logout : undefined} onNewChat={newChat} />
    </Animated.View>}
    {!ready && <View pointerEvents="none" accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={styles.preload}>
      {Object.entries(preloadAssets).map(([key, source]) => <Image key={key} source={source} accessible={false} style={styles.preloadImage}
        onLoad={() => setLoaded(previous => new Set(previous).add(key))} onError={() => setAssetError(true)} />)}
    </View>}
  </View>;
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#fff' },
  demoControls: { paddingHorizontal: 20 }, errorSwitch: { minWidth: layout.touchTarget, minHeight: layout.touchTarget, justifyContent: 'center', paddingVertical: 8 }, demoControlText: { fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 17, color: '#666' },
  content: { flexGrow: 1, width: '100%', maxWidth: layout.contentWidth, alignSelf: 'center' },
  notice: { paddingHorizontal: 24, paddingVertical: 4, fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 17, color: '#666', textAlign: 'center' },
  loading: { flexGrow: 1, padding: 24, justifyContent: 'center' },
  body: { fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 20, color: '#142033', textAlign: 'center' },
  landing: { flexGrow: 1, justifyContent: 'center', alignItems: 'center', paddingHorizontal: 24, paddingVertical: 24, gap: 28 },
  greeting: { width: '100%', alignItems: 'center', gap: 12 },
  avatar: { width: 62, height: 62, borderRadius: 31, overflow: 'hidden', borderWidth: 1, borderColor: '#f1f5f9', boxShadow: '0px 2px 8px rgba(0,0,0,0.03)' },
  // Figma 683:2451: exact crop of the original image, not a replacement avatar.
  avatarImage: { position: 'absolute', width: 62 * 1.1084, height: 62 * 1.0412, left: -62 * 0.0438, top: -62 * 0.0507 },
  greetingText: { width: '100%', alignItems: 'center', gap: 6 },
  hello: { fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 20, color: '#666', textAlign: 'center' },
  heading: { width: '100%', fontFamily: 'Fraunces_700Bold', fontSize: 26, lineHeight: 30, letterSpacing: -0.3, color: '#142033', textAlign: 'center' },
  suggestions: { maxWidth: '100%', alignItems: 'center', gap: 10 },
  suggestion: { maxWidth: '100%', minHeight: layout.touchTarget, minWidth: layout.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 8,
    paddingHorizontal: 16, paddingVertical: 10, borderRadius: 28, borderWidth: 1, borderColor: '#e5e7eb', backgroundColor: '#fff' },
  messageIcon: { width: 16, height: 16, flexShrink: 0 },
  suggestionText: { flexShrink: 1, fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 20, color: '#142033' },
  preload: { position: 'absolute', width: 1, height: 1, opacity: 0 }, preloadImage: { width: 1, height: 1 },
});
