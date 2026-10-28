// Actual React callbacks with synthetic native controls. No Android/TalkBack proof.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const {fixture,flags,nodes,flat}=require('./check-chat-home-preview.cjs');
const action=(f,label)=>nodes(f.tree,'Pressable').find(n=>n.props.accessibilityLabel===label);
const list=f=>nodes(f.tree,'ScrollView').find(n=>n.props.testID==='conversation-messages');
const contentChange=f=>{list(f).props.onContentSizeChange();f.draw();};
function respond(f){const callbacks=[...f.rt.timers.values()];f.rt.timers.clear();callbacks.forEach(fn=>fn());f.draw();}
function start(f,prompt='How do I renew my NIC?'){action(f,prompt).props.onPress();f.draw();}
for(const mode of ['standalone','demo']){
  flags(true,'1');if(mode==='demo')process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO='1';
  const f=fixture(400,772,1,{mode});f.set('Unrelated landing draft');start(f);assert(list(f));assert(f.text().includes('Preparing local mock response…'));
  const originalTimer=[...f.rt.timers.values()][0];contentChange(f);assert(f.scrolls.some(p=>p.end));
  f.set('Draft while responding');
  list(f).props.onScrollBeginDrag();
  list(f).props.onScroll({nativeEvent:{contentOffset:{y:20},contentSize:{height:2000},layoutMeasurement:{height:400}}});
  list(f).props.onScrollEndDrag();
  const before=f.scrolls.length;respond(f);contentChange(f);assert.equal(f.scrolls.length,before,'reading history must not be pulled to bottom');assert(action(f,'Jump to latest messages'));
  assert.equal(nodes(f.tree,'TextInput')[0].props.value,'Draft while responding');
  const jumpButton=action(f,'Jump to latest messages'),jumpStyle=flat(jumpButton.props.style);
  assert.equal(jumpStyle.width,48);assert.equal(jumpStyle.height,48);assert(jumpStyle.minWidth>=48&&jumpStyle.minHeight>=48);
  assert.equal(jumpStyle.flexShrink,0);assert.equal(jumpStyle.position,undefined,'jump reserves space instead of covering content');
  assert.equal(nodes(jumpButton,'Text').length,0,'icon-only visible control');
  const jumpImage=nodes(jumpButton,'Image')[0],iconStyle=flat(jumpImage.props.style);
  assert.equal(jumpImage.props.accessible,false);assert.equal(iconStyle.width,20);assert.equal(iconStyle.height,20);assert.deepEqual(iconStyle.transform,[{rotate:'-90deg'}]);
  assert(!nodes(list(f),'Pressable').includes(jumpButton),'jump sits outside the scrolling messages, above composer');
  action(f,'Jump to latest messages').props.onPress();f.draw();assert(f.scrolls.at(-1).end);assert(!action(f,'Jump to latest messages'));
  assert(f.text().includes('Mock service details · Unverified'));assert(f.text().includes('Official website · Demo reference, not verification of this response'));
  assert.equal(nodes(f.tree).filter(n=>n.props.accessibilityRole==='link').length,1);assert(action(f,'Start renewal. Unavailable in this local demo.').props.disabled);
  const back=action(f,'Back to Chat Home').props.onPress;back();f.draw();assert(!list(f));assert.equal(nodes(f.tree,'TextInput')[0].props.value,'Unrelated landing draft');
  originalTimer();f.draw();assert(!list(f));assert.equal(f.rt.timers.size,0);
  // Blank local Send stays on landing with an accessible error.
  f.set('   ');action(f,'Send local message').props.onPress();f.draw();assert(!list(f));assert(f.text().includes('Enter a message.'));assert(nodes(f.tree,'Text').some(n=>n.props.accessibilityRole==='alert'));
  f.set('A typed local question');const submit=action(f,'Send local message').props.onPress;submit();submit();f.draw();assert.equal(f.rt.timers.size,1);respond(f);
  assert(f.text().includes('A typed local question'));assert(!f.text().includes('NIC renewal · Sample'));
  f.set('   ');action(f,'Send local message').props.onPress();f.draw();assert(f.text().includes('Enter a message.'));
  action(f,'Simulate next response error').props.onPress();f.draw();f.set('A follow-up');const send=action(f,'Send local message').props.onPress;send();send();f.draw();
  assert.equal(f.rt.timers.size,1);assert(action(f,'Send local message').props.disabled);respond(f);assert(action(f,'Retry local mock response'));
  const retry=action(f,'Retry local mock response').props.onPress;retry();retry();f.draw();assert.equal(f.rt.timers.size,1);respond(f);
  assert.equal(f.text().filter(t=>t==='A follow-up').length,1,'Retry must not duplicate citizen message');send();f.draw();assert.equal(f.rt.timers.size,0,'old Send cannot replay after response');
  f.set('Pending draft');action(f,'Send local message').props.onPress();f.draw();const late=[...f.rt.timers.values()][0];
  f.env.visible=true;f.env.back();assert(list(f));f.env.back();f.draw();assert(!list(f));assert.equal(f.rt.timers.size,0);late();f.draw();assert(!list(f));
  assert.equal(nodes(f.tree,'TextInput')[0].props.value,'','submitted landing draft was cleared');
  start(f);const staleWork=[...f.rt.timers.values()][0];action(f,'New chat').props.onPress();f.draw();assert(!list(f));assert.equal(f.rt.timers.size,0);staleWork();f.draw();assert(!list(f));
  f.set('New chat clears this');action(f,'Open menu').props.onPress();f.draw();action(f,'New chat').props.onPress();f.draw();f.rt.flush();f.draw();assert.equal(nodes(f.tree,'TextInput')[0].props.value,'');
  if(mode==='demo'){start(f);action(f,'Back to Chat Home').props.onPress();f.draw();action(f,'Open menu').props.onPress();f.draw();const logout=action(f,'Logout').props.onPress;logout();logout();f.draw();assert.equal(f.exits,1);assert.equal(f.rt.timers.size,0);assert(!list(f));}
  f.unmount();
  // An earlier landing visit's callback must not clear a later landing draft.
  const visits=fixture(400,772,1,{mode});
  action(visits,'Open menu').props.onPress();visits.draw();const oldNewChat=action(visits,'New chat').props.onPress;
  action(visits,'Close menu').props.onPress();visits.draw();visits.rt.flush();visits.draw();
  start(visits);action(visits,'Back to Chat Home').props.onPress();visits.draw();visits.set('Newer landing draft');
  oldNewChat();visits.draw();assert.equal(nodes(visits.tree,'TextInput')[0].props.value,'Newer landing draft','stale landing New chat cannot erase a newer draft');
  action(visits,'Open menu').props.onPress();visits.draw();action(visits,'New chat').props.onPress();visits.draw();visits.rt.flush();visits.draw();
  assert.equal(nodes(visits.tree,'TextInput')[0].props.value,'','current visit New chat intentionally clears the draft');
  assert(!nodes(visits.tree,'View').some(n=>n.props.accessibilityViewIsModal));
  visits.set('Draft after intentional reset');oldNewChat();visits.draw();assert.equal(nodes(visits.tree,'TextInput')[0].props.value,'Draft after intentional reset');visits.unmount();
  const pending=fixture(400,772,1,{mode});start(pending);const stale=[...pending.rt.timers.values()][0];pending.unmount();stale();assert.equal(pending.rt.timers.size,0);
}
const root=path.resolve(__dirname,'..');
// Native callbacks may arrive in either order; only user movement changes follow intent.
const scrollEvent=(y,height=1000,viewport=400)=>({nativeEvent:{contentOffset:{y},contentSize:{height},layoutMeasurement:{height:viewport}}});
const ends=f=>f.scrolls.filter(p=>p.source==='conversation-messages'&&p.end).length;
function scrollFixture(){flags(true,'1');const f=fixture();start(f);list(f).props.onScroll(scrollEvent(600));contentChange(f);list(f).props.onScroll(scrollEvent(600));return f;}
for(const order of ['scroll-first','size-first'])for(const older of [false,true]){
  const f=scrollFixture();
  if(older){list(f).props.onScrollBeginDrag();list(f).props.onScroll(scrollEvent(100));list(f).props.onScrollEndDrag();}
  const stale=list(f).props,before=ends(f);respond(f);
  if(order==='scroll-first')list(f).props.onScroll(scrollEvent(older?100:600,1800));
  contentChange(f);
  if(order==='size-first')list(f).props.onScroll(scrollEvent(older?100:600,1800));
  assert.equal(ends(f)-before,older?0:1,order+' preserves pre-growth follow state');
  assert.equal(!!action(f,'Jump to latest messages'),older);
  const count=ends(f);stale.onContentSizeChange();stale.onScroll(scrollEvent(0,9999));assert.equal(ends(f),count,'stale revision does not jump');
  if(older){action(f,'Jump to latest messages').props.onPress();f.draw();list(f).props.onScroll(scrollEvent(1400,1800));}
  else list(f).props.onScroll(scrollEvent(1400,1800));
  // Keyboard/layout-only size and offset notifications do not jump or change reader intent.
  const layoutCount=ends(f);list(f).props.onScroll(scrollEvent(1400,1800,200));contentChange(f);assert.equal(ends(f),layoutCount);
  f.set('Rapid update');action(f,'Send local message').props.onPress();f.draw();const oldSize=list(f).props.onContentSizeChange;
  respond(f);const rapidCount=ends(f);oldSize();assert.equal(ends(f),rapidCount);contentChange(f);assert.equal(ends(f),rapidCount+1,'rapid coalesced updates keep following');
  const departed=list(f).props;action(f,'Back to Chat Home').props.onPress();f.draw();const exitCount=ends(f);departed.onContentSizeChange();assert.equal(ends(f),exitCount);f.unmount();departed.onContentSizeChange();assert.equal(ends(f),exitCount);
}
{
  const f=scrollFixture();respond(f);const before=ends(f);
  // A deliberate drag after capture but before native layout takes precedence.
  list(f).props.onScrollBeginDrag();list(f).props.onScroll(scrollEvent(100,1800));list(f).props.onScrollEndDrag();contentChange(f);
  assert.equal(ends(f),before);assert(action(f,'Jump to latest messages'));f.unmount();
}
{
  const f=scrollFixture();respond(f);contentChange(f);list(f).props.onScroll(scrollEvent(1400,1800));
  // Accessibility scrolling need not emit drag callbacks.
  list(f).props.onScroll(scrollEvent(1000,1800));f.set('Accessibility reader');action(f,'Send local message').props.onPress();f.draw();
  const before=ends(f);respond(f);contentChange(f);assert.equal(ends(f),before);assert(action(f,'Jump to latest messages'));f.unmount();
}
const assetDir=path.join(root,'assets/figma/chat-conversation'),digest=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
for(const [file,expected] of Object.entries(JSON.parse(fs.readFileSync(path.join(assetDir,'sha256.json'),'utf8'))))assert.equal(digest(path.join(assetDir,file)),expected);
for(const file of ['plus','mic','send','home','services','requests'])assert.equal(digest(path.join(assetDir,file+'.png')),digest(path.join(root,'assets/figma/chat-home',file+'.png')),'shared image must match Figma exactly');
for(const file of ['chatDemo.ts','components/ChatConversation.tsx','components/ChatMessage.tsx','components/ChatComposer.tsx']){
  const source=fs.readFileSync(path.join(root,file),'utf8');assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|Linking)\b/.test(source));assert(!/allowFontScaling=\{false\}|maxFontSizeMultiplier/.test(source));
}
for(const [w,h]of [[320,240],[320,568],[400,772],[412,915],[600,960]])for(const scale of [1,2,3]){
  flags(true,'1');const f=fixture(w,h,scale);start(f);respond(f);
  let chrome=0;
  for(const [id,height]of [['chat-header',56],['chat-notices',80*scale],['chat-demo-controls',48*scale],['chat-composer',84*scale],['chat-navigation',64*scale]]){
    nodes(f.tree,'View').find(n=>n.props.testID===id).props.onLayout({nativeEvent:{layout:{height}}});chrome+=height;
  }
  f.draw();let shell=nodes(f.tree,'ScrollView').find(n=>n.props.testID==='chat-shell');assert(flat(shell.props.contentContainerStyle).minHeight>=chrome+96,'intrinsic controls remain reachable by shell overflow');
  nodes(f.tree,'TextInput')[0].props.onFocus();const before=f.scrolls.filter(p=>p.source==='conversation-messages').length;
  f.env.visible=true;f.env.listeners.keyboardDidShow({endCoordinates:{screenY:200}});f.rt.flush();f.draw();assert.equal(f.scrolls.filter(p=>p.source==='conversation-messages').length,before,'keyboard must not scroll reading history');
  f.measure(176,100);shell=nodes(f.tree,'ScrollView').find(n=>n.props.testID==='chat-shell');assert.equal(flat(shell.props.contentContainerStyle).paddingBottom,0,'resized IME has no duplicate padding');
  f.env.visible=false;f.env.listeners.keyboardDidHide();f.draw();
  for(const n of nodes(f.tree,'Image')){const style=flat(n.props.style);assert(style.width>0&&style.height>0);}
  for(const n of nodes(f.tree,'Pressable'))assert(flat(n.props.style).minHeight>=48,'minimum touch height');
  assert(nodes(f.tree,'Text').some(n=>[n.props.children].flat(Infinity).includes('Not verified')));f.unmount();
}
if(process.argv.includes('--export')){
  const dir=path.resolve(root,process.argv[process.argv.indexOf('--export')+1]),files=[];function walk(d){for(const e of fs.readdirSync(d,{withFileTypes:true})){const f=path.join(d,e.name);e.isDirectory()?walk(f):files.push(f);}}walk(dir);
    for(const file of files.filter(f=>/\.(hbc|js)$/.test(f)))for(const marker of ['ChatConversation','Preparing local mock response','Simulate next response error','Mock service details','localAnswer','OfficialSourceReference','Official website · Demo reference, not verification of this response','https://drp.gov.lk/'])for(const encoding of ['utf8','utf16le'])assert(!fs.readFileSync(file).includes(Buffer.from(marker,encoding)),marker+' leaked');
  const hash=f=>crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex'),hashes=new Set(files.map(hash));for(const asset of ['id-card.png','profile-bell.png'])assert(!hashes.has(hash(path.join(root,'assets/figma/chat-conversation',asset))));
  console.log('PASS conversation production exclusion:',dir);
}
async function sourceLinkChecks(){
  const label='Official website · Demo reference, not verification of this response';
  const link=f=>nodes(f.tree,'Pressable').find(n=>n.props.accessibilityRole==='link');
  function deferred(){let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});return{promise,resolve,reject};}
  for(const result of ['success','failure','throw']){
    const gate=deferred();let attempts=0;const f=fixture(400,772,1,{sourceOnly:true,openURL(){if(++attempts>1)return Promise.resolve();if(result==='throw')throw Error('handoff failed');return gate.promise;}});
    assert(f.text().includes(label));assert(link(f).props.accessibilityLabel.includes('Department for Registration of Persons'));
    assert.equal(link(f).props.accessibilityHint,'Opens in your external browser.');
    assert(flat(link(f).props.style).minHeight>=48);assert(flat(link(f).props.style).minWidth>=48);
    const open=link(f).props.onPress,pending=open();if(result!=='throw')open();f.draw();
    assert.deepEqual(f.links,[['https://drp.gov.lk/']]);
    if(result!=='throw'){assert(link(f).props.accessibilityState.busy);assert(link(f).props.disabled);result==='success'?gate.resolve():gate.reject(Error('no browser'));}
    await pending;f.draw();assert.equal(link(f).props.disabled,false);
    const error='Couldn’t open the official website. Please try again.';
    if(result==='success')assert(!f.text().includes(error));else{
      assert(f.text().includes(error));assert(nodes(f.tree,'Text').some(n=>n.props.accessibilityRole==='alert'&&n.props.accessibilityLiveRegion==='polite'));
      await link(f).props.onPress();f.draw();assert.equal(f.links.length,2,'retry remains available');assert(!f.text().includes(error),'successful retry clears the error');
    }
    f.unmount();const count=f.links.length;await open();assert.equal(f.links.length,count,'departed link cannot reopen browser');
  }
  for(const outcome of ['resolve','reject']){
    const gate=deferred(),f=fixture(400,772,1,{sourceOnly:true,openURL:()=>gate.promise});
    const pending=link(f).props.onPress();f.unmount();const writes=f.rt.writes;gate[outcome](outcome==='reject'?Error('late failure'):undefined);await pending;assert.equal(f.rt.writes,writes,'late completion cannot update an unmounted reference');
  }
  for(const mode of ['standalone','demo']){
    flags(true,'1');if(mode==='demo')process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO='1';
    const gate=deferred(),f=fixture(400,772,1,{mode,openURL:()=>gate.promise});start(f);respond(f);f.set('Private local draft');
    list(f).props.onScrollBeginDrag();list(f).props.onScroll(scrollEvent(100,1800));list(f).props.onScrollEndDrag();
    const beforeText=f.text(),beforeScroll=f.scrolls.length,beforeClears=f.clears;
    const pending=link(f).props.onPress();link(f).props.onPress();f.draw();assert.deepEqual(f.links,[['https://drp.gov.lk/']]);gate.resolve();await pending;f.draw();
    assert.equal(nodes(f.tree,'TextInput')[0].props.value,'Private local draft');assert.deepEqual(f.text(),beforeText);assert.equal(f.scrolls.length,beforeScroll);assert.equal(f.clears,beforeClears);assert.equal(f.exits,0);f.unmount();
  }
  const source=fs.readFileSync(path.join(root,'components/OfficialSourceReference.tsx'),'utf8');
  assert(source.includes("const OFFICIAL_URL = 'https://drp.gov.lk/';"));assert(source.includes('Linking.openURL(OFFICIAL_URL)'));
  assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|console)\b/.test(source));
  console.log('PASS official reference: fixed URL/disclaimer, accessible 48dp link, success/reject/throw/retry, duplicate taps, late/unmounted callbacks, both-mode draft/conversation/scroll retention. Browser/TalkBack behavior needs Android testing.');
}
sourceLinkChecks().catch(error=>{console.error(error);process.exitCode=1;});
console.log('PASS conversation: both modes, suggestions/typed Send, blank errors, local responding/error/Retry, duplicate/stale events, cancellation, drafts, Back/New chat/Logout, reading-position retention/Jump, disabled services, 15 synthetic viewports. No Android/device testing.');
