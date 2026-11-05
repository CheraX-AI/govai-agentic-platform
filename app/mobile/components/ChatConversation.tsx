import { useEffect, useRef, useState, type RefObject } from 'react';
import { Image, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { conversationAssets, type LocalThread } from '../chatDemo';
import { layout } from '../layout';
import { AiResponse, CitizenMessage } from './ChatMessage';
export type ChatFollowController = { threadId: number | null; revision: number; capture?: () => void };
export function ChatConversation({ thread, pending, failed, onRetry, follow, live = false }: { live?: boolean; thread: LocalThread; pending: boolean; failed: boolean; onRetry: () => void; follow: RefObject<ChatFollowController> }) {
  const scroll = useRef<ScrollView>(null);
  const nearBottom = useRef(true);
  const [unseen, setUnseen] = useState(false);
  const previousCount = useRef(thread.messages.length);
  const snapshot = useRef<boolean | null>(null);
  const initialLayout = useRef(true);
  const gesture = useRef(false);
  const settling = useRef(false);
  const metrics = useRef<{ y: number; height: number; viewport: number } | null>(null);
  const revision = follow.current.revision;
  const active = () => follow.current.threadId === thread.id && follow.current.revision === revision;
  useEffect(() => {
    const capture = () => { if(follow.current.threadId===thread.id && snapshot.current===null)snapshot.current=nearBottom.current; };
    follow.current.capture=capture;
    return () => { if(follow.current.capture===capture)follow.current.capture=undefined; };
  }, [thread.id, follow]);
  const scrollToLatest = () => { settling.current=true; scroll.current?.scrollToEnd({animated:false}); };
  const jump = () => { if(!active())return; nearBottom.current = true; if(snapshot.current!==null)snapshot.current=true; setUnseen(false); scrollToLatest(); };
  return <View style={[styles.root, { minHeight: 96 }]}>
    <ScrollView ref={scroll} testID="conversation-messages" style={styles.root} contentContainerStyle={styles.messages}
      keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag" automaticallyAdjustKeyboardInsets={false}
      automaticallyAdjustContentInsets={false} contentInsetAdjustmentBehavior="never"
      onScrollBeginDrag={() => { if(active()){gesture.current=true;settling.current=false;} }}
      onScrollEndDrag={() => { if(active())gesture.current=false; }}
      onMomentumScrollBegin={() => { if(active())gesture.current=true; }}
      onMomentumScrollEnd={() => { if(active())gesture.current=false; }}
      onScroll={event => { const {contentOffset,contentSize,layoutMeasurement}=event.nativeEvent;
        if(!active())return;
        const previous=metrics.current, next={y:contentOffset.y,height:contentSize.height,viewport:layoutMeasurement.height};
        metrics.current=next;
        const atBottom=next.height-next.viewport-next.y<=96;
        const moved=!previous || Math.abs(next.y-previous.y)>.5;
        const sameGeometry=previous && next.height===previous.height && next.viewport===previous.viewport;
        const movedAway=previous && next.y<previous.y-.5;
        // Layout/content notifications cannot rewrite pre-update reader intent.
        // Offset movement toward older content also covers accessibility scrolling without drag events.
        if(settling.current && !gesture.current){
          if(atBottom){settling.current=false;return;}
          if(!movedAway)return;
          settling.current=false;
        }
        if(moved && (gesture.current || (!previous || sameGeometry) || movedAway)){
          nearBottom.current=atBottom;
          if(snapshot.current!==null)snapshot.current=atBottom;
        }
        if(nearBottom.current)setUnseen(false);
      }} scrollEventThrottle={16}
      onContentSizeChange={() => {
        if(!active())return;
        const changed = previousCount.current !== thread.messages.length;
        previousCount.current=thread.messages.length;
        const shouldFollow=snapshot.current ?? (initialLayout.current && nearBottom.current);
        snapshot.current=null; initialLayout.current=false;
        if(shouldFollow){setUnseen(false);scrollToLatest();}else if(changed)setUnseen(true);
      }}>
      <View style={styles.date}><View style={styles.divider}/><Text style={styles.caption}>{live ? 'Today' : 'Today \u00b7 Local demo'}</Text><View style={styles.divider}/></View>
      {thread.messages.map(message => message.role==='citizen' ? <CitizenMessage key={message.id} message={message}/> : <AiResponse key={message.id} message={message}/>)}
      {pending && <Text accessibilityRole="progressbar" accessibilityLiveRegion="polite" style={styles.body}>{live ? 'Preparing response...' : 'Preparing local mock response...'}</Text>}
      {failed && <View style={styles.error}><Text accessibilityRole="alert" accessibilityLiveRegion="polite" style={styles.body}>{live ? 'The response could not be retrieved. Your message is kept. Try again.' : 'The local mock response could not be prepared. Your message is kept. Try again.'}</Text>
        <Pressable onPress={onRetry} accessibilityRole="button" accessibilityLabel={live ? "Retry response" : "Retry local mock response"} style={styles.action}><Text style={styles.actionText}>Retry</Text></Pressable></View>}
    </ScrollView>
    {unseen && !nearBottom.current && <Pressable onPress={jump} accessibilityRole="button" accessibilityLabel="Jump to latest messages" style={styles.jumpButton}>
      <Image source={conversationAssets.back} accessible={false} resizeMode="contain" style={styles.jumpIcon} />
    </Pressable>}
  </View>;
}
const styles=StyleSheet.create({root:{flex:1,minHeight:0},messages:{paddingHorizontal:20,paddingVertical:12,gap:20},
  date:{flexDirection:'row',alignItems:'center',gap:12},divider:{height:1,flex:1,backgroundColor:'#e5e7eb'},caption:{fontFamily:'Inter_400Regular',fontSize:11,lineHeight:17,color:'#666'},
  body:{fontFamily:'Inter_400Regular',fontSize:14,lineHeight:21,color:'#142033'},error:{gap:8},
  action:{minHeight:layout.touchTarget,justifyContent:'center',alignItems:'center',padding:12,borderWidth:1,borderColor:'#a30f2d',borderRadius:12,marginHorizontal:20},
  actionText:{fontFamily:'Inter_600SemiBold',fontSize:14,lineHeight:21,color:'#a30f2d'},
  jumpButton:{width:layout.touchTarget,height:layout.touchTarget,minWidth:layout.touchTarget,minHeight:layout.touchTarget,flexShrink:0,alignSelf:'center',alignItems:'center',justifyContent:'center',borderWidth:1,borderColor:'#a30f2d',borderRadius:24,backgroundColor:'#fff'},
  jumpIcon:{width:20,height:20,transform:[{rotate:'-90deg'}]},
});
