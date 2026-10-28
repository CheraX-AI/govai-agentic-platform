// Actual callbacks with mocked native controls/animations. Not a TalkBack/device test.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const {fixture,flags,nodes,flat}=require('./check-chat-home-preview.cjs');
const keys=['EXPO_PUBLIC_CHAT_HOME_PREVIEW','EXPO_PUBLIC_CONNECTED_UI_DEMO'],saved=keys.map(k=>process.env[k]),dev=global.__DEV__;
const action=(f,label)=>nodes(f.tree,'Pressable').find(n=>n.props.accessibilityLabel===label);
const overlay=f=>nodes(f.tree,'View').find(n=>n.props.accessibilityViewIsModal);
function settle(f){f.draw();f.rt.flush();f.draw();}
function open(f){action(f,'Open menu').props.onPress();f.draw();action(f,'Close menu').props.onLayout();settle(f);}
try{
  for(const mode of ['standalone','demo'])for(const method of ['Close menu','Close menu, outside area','back']){
    flags(true,'1');if(mode==='demo')process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO='1';
    const f=fixture(400,772,1,{mode});f.set('Retained local draft');
    const input=nodes(f.tree,'TextInput')[0].props.ref.current;
    nodes(f.tree,'TextInput')[0].props.onFocus();f.env.visible=true;open(f);
    assert.equal(f.env.visible,false);assert.equal(f.blurs,1);assert.equal(f.clears,0);
    assert.equal(nodes(f.tree,'TextInput')[0].props.ref.current,input,'same native input remains mounted');
    assert.equal(nodes(f.tree,'TextInput')[0].props.value,'Retained local draft');assert.equal(nodes(f.tree,'TextInput')[0].props.editable,false);
    assert(nodes(f.tree,'View').some(n=>n.props.pointerEvents==='none'&&n.props.accessibilityElementsHidden===true&&n.props.importantForAccessibility==='no-hide-descendants'));
    assert(f.events.some(([label,event])=>label==='Close menu'&&event==='focus'));
    const menu=overlay(f);assert(menu);const controls=nodes(menu,'Pressable');assert.equal(controls.filter(n=>n.props.disabled).length,mode==='demo'?8:9);
    for(const n of controls.filter(n=>n.props.disabled)){assert(n.props.accessibilityState.disabled);assert(!n.props.onPress);}
    const text=nodes(menu,'Text').flatMap(n=>[n.props.children].flat(Infinity));
    for(const value of ['New chat','Recent chats','My applications','Today','Yesterday','Citizen','Development preview','Sample chat history · Actions unavailable'])assert(text.includes(value));
    assert(!text.some(t=>typeof t==='string'&&/Rathishan|GovAI Plus|RM/.test(t)));
    if(method==='back'){f.env.visible=true;f.env.back();}else action(f,method).props.onPress();settle(f);
    assert(!overlay(f));assert.equal(f.exits,0);assert.equal(f.clears,0);assert.equal(nodes(f.tree,'TextInput')[0].props.value,'Retained local draft');
    assert.equal(nodes(f.tree,'TextInput')[0].props.editable,true);assert.equal(f.focuses.at(-1),'Open menu');assert.equal(f.events.at(-1)[0],'Open menu');
    // Changing motion preferences after closing must not steal input focus.
    const count=f.focuses.length;f.motionListeners.reduceMotionChanged(false);settle(f);assert.equal(f.focuses.length,count);
    for(let i=0;i<3;i++){open(f);action(f,'Close menu').props.onPress();settle(f);}
    assert.equal(f.clears,0);assert.equal(nodes(f.tree,'TextInput')[0].props.ref.current,input);
    f.env.visible=true;f.env.back();assert.equal(f.exits,0);f.env.back();f.draw();assert.equal(f.exits,1);assert.equal(nodes(f.tree,'TextInput')[0].props.value,'');f.unmount();
  }
  flags(true,'1');let layouts=0;
  // Logout clears before exit and cannot be replayed after leaving/unmount.
  process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO='1';
  for(const pendingFocus of [false,true]){
    let f,cleanAtExit=false;
    f=fixture(400,772,1,{mode:'demo',onExit(){f.draw();cleanAtExit=!overlay(f)&&nodes(f.tree,'TextInput')[0].props.value===''&&!f.env.visible;}});
    f.set('Temporary draft');open(f);
    const close=action(f,'Close menu').props,opener=action(f,'Open menu').props.onPress;
    if(pendingFocus)close.onLayout();
    const count=f.events.length,logout=action(f,'Logout');assert.equal(logout.props.disabled,false);assert.equal(logout.props.accessibilityState.disabled,false);
    f.env.visible=true;logout.props.onPress();logout.props.onPress();close.onPress();opener();settle(f);
    assert(cleanAtExit);assert.equal(f.exits,1);assert.equal(f.clears,1);assert.equal(f.events.length,count,'no return focus on logout');
    assert.equal(f.rt.frames.size,0);f.env.back();assert.equal(f.exits,1);f.unmount();logout.props.onPress();assert.equal(f.exits,1);
  }
  const standalone=fixture();open(standalone);assert(action(standalone,'Logout').props.disabled);assert.equal(action(standalone,'Logout').props.onPress,undefined);
  assert(standalone.text().includes('Unavailable: no login occurred in this preview.'));standalone.unmount();
  const stale=fixture(400,772,1,{mode:'demo'});open(stale);const staleLogout=action(stale,'Logout').props.onPress;stale.unmount();staleLogout();assert.equal(stale.exits,0);
  for(const [w,h]of [[320,240],[320,568],[360,640],[360,800],[400,772],[412,915],[600,960]])for(const scale of [1,2,3]){
    const f=fixture(w,h,scale);f.env.insets={top:44,bottom:24,left:0,right:0};open(f);
    const menu=overlay(f),drawer=nodes(menu,'View').find(n=>flat(n.props.style).borderTopRightRadius===24),style=flat(drawer.props.style);
    assert.equal(style.width,Math.min(340,w-48));assert(w-style.width>=48);assert.equal(style.top,0);assert.equal(style.bottom,0);
    const region=id=>nodes(overlay(f)).find(n=>n.props.testID===id);
    const shell=region('drawer-shell'),middle=region('drawer-middle'),content=flat(shell.props.contentContainerStyle);
    assert.equal(shell.props.scrollEnabled,false);assert.equal(middle.props.scrollEnabled,true);
    assert.equal(content.paddingTop,48);assert.equal(content.paddingBottom,32);assert.equal(content.paddingLeft,20);
    assert(!nodes(middle).includes(region('drawer-header')));assert(!nodes(middle).includes(region('drawer-footer')));
    assert(nodes(region('drawer-footer'),'Pressable').some(n=>n.props.accessibilityLabel==='Logout'));
    const measure=(node,height)=>node.props.onLayout({nativeEvent:{layout:{height}}});
    measure(drawer,h);measure(region('drawer-header'),56);measure(region('drawer-footer'),130*scale);f.draw();
    const fallback=56+130*scale+48+32+48>h;
    assert.equal(region('drawer-shell').props.scrollEnabled,fallback);assert.equal(region('drawer-middle').props.scrollEnabled,!fallback);
    // Natural fixed-region measurements remain stable when switching scroll ownership.
    measure(region('drawer-footer'),h);f.draw();assert(region('drawer-shell').props.scrollEnabled);assert.equal(region('drawer-middle').props.scrollEnabled,false);
    measure(region('drawer-footer'),48);f.draw();assert.equal(region('drawer-shell').props.scrollEnabled,false);
    assert.equal(flat(action(f,'Close menu').props.style).minHeight,48);
    assert.equal(flat(action(f,'Logout').props.style).minHeight,48);assert.equal(flat(action(f,'Logout').props.style).minWidth,48);
    for(const n of nodes(menu,'Image')){const s=flat(n.props.style);assert(s.width>0&&s.height>0);}
    for(const [label,size]of [['New chat',16],['Passport application process',15],['Citizen',14],['Today',13],['Logout',16]]){
      const text=nodes(menu,'Text').find(n=>[n.props.children].flat(Infinity).includes(label));assert.equal(flat(text.props.style).fontSize,size);assert.notEqual(text.props.allowFontScaling,false);
    }
    action(f,'Close menu').props.onPress();settle(f);f.unmount();layouts++;
  }
  // Close an opening fade and unmount with focus work pending: no queued callbacks remain.
  const f=fixture();f.motionListeners.reduceMotionChanged(false);f.draw();open(f);assert(f.timings.some(t=>t.duration===200));action(f,'Close menu').props.onPress();f.draw();assert(f.timings.some(t=>t.duration===250));f.unmount();
  const root=path.resolve(__dirname,'..'),dir=path.join(root,'assets/figma/chat-home-menu');
  const manifest=JSON.parse(fs.readFileSync(path.join(dir,'sha256.json'),'utf8'));
  const hash=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
  for(const [file,expected]of Object.entries(manifest))assert.equal(hash(path.join(dir,file)),expected);
  if(process.argv.includes('--export')){
    const exportDir=path.resolve(root,process.argv[process.argv.indexOf('--export')+1]),files=[];
    function walk(d){for(const e of fs.readdirSync(d,{withFileTypes:true})){const f=path.join(d,e.name);e.isDirectory()?walk(f):files.push(f);}}walk(exportDir);
    const artifactHashes=new Set(files.map(hash));for(const name of Object.keys(manifest).filter(n=>n.endsWith('.png')))assert(!artifactHashes.has(manifest[name]),name+' leaked into production');
    for(const file of files.filter(f=>/\.(hbc|js)$/.test(f)))for(const marker of ['ChatHomeMenu','Sample chat history','Close menu, outside area'])for(const encoding of ['utf8','utf16le'])assert(!fs.readFileSync(file).includes(Buffer.from(marker,encoding)),marker+' leaked into production');
    console.log('PASS menu module/copy/assets excluded from production:',exportDir);
  }
  console.log(`PASS menu: both modes, draft/native input retention, keyboard dismissal, X/outside/Back, background isolation, focus restoration, reduced motion, disabled unimplemented destinations, ${layouts} synthetic layouts, asset hashes. No device/TalkBack rendering.`);
}finally{global.__DEV__=dev;keys.forEach((k,i)=>saved[i]===undefined?delete process.env[k]:process.env[k]=saved[i]);}
