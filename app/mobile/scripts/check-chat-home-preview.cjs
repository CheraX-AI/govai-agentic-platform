// Actual React callbacks with mocked native APIs. Optional HTML rendering is a
// browser layout approximation, never evidence of Android/TalkBack behavior.
const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict');
const crypto = require('node:crypto'), vm = require('node:vm');
global.__DEV__ = false;
const { runtime, environment, loader, nodes } = require('./check-register.cjs');
const root = path.resolve(__dirname, '..');
const keys = ['EXPO_PUBLIC_CHAT_HOME_PREVIEW','EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW','EXPO_PUBLIC_CHECK_EMAIL_PREVIEW','EXPO_PUBLIC_RESET_PASSWORD_PREVIEW','EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW','EXPO_PUBLIC_CONNECTED_UI_DEMO'];
const saved = keys.map(k => process.env[k]), savedDev = global.__DEV__;
const flat = s => Object.assign({}, ...[s].flat(Infinity).filter(Boolean));
const read = f => fs.readFileSync(path.join(root,f),'utf8');
const source = read('ChatHomePreviewScreen.tsx') + read('components/ChatHomeContent.tsx');
assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|Linking)\b/.test(source));
assert(!/allowFontScaling=\{false\}|maxFontSizeMultiplier|numberOfLines|maxLength/.test(source));
assert(!/CHAT_HOME_PREVIEW/.test(read('LoginScreen.tsx')),'Login cannot open standalone preview');
assert(!/Rathishan/.test(source),'no invented display name');
function flags(dev, value) { global.__DEV__=dev;keys.forEach(k=>process.env[k]='0');if(value===undefined)delete process.env[keys[0]];else process.env[keys[0]]=value; }
function expand(t) {
  if (!t || typeof t !== 'object') return t;
  if (typeof t.type === 'function') return expand(t.type(t.props));
  return {...t,props:{...t.props,children:[t.props?.children].flat(Infinity).map(expand)}};
}
function fixture(width=400,height=772,fontScale=1,{fontError=false,assetError=false,displayName,mode='standalone',onExit:exitCallback,sourceOnly=false,openURL=()=>Promise.resolve()}={}) {
  const rt=runtime(),env=environment(width,height,fontScale),overrides={};
  const focuses=[],events=[],timings=[],motionListeners={};let blurs=0;
  const Animated={Value:class {constructor(value){this.value=value;}setValue(value){this.value=value;}stopAnimation(){}},View:'AnimatedView',timing(value,options){timings.push(options);return{start(callback){value.value=options.toValue;callback?.({finished:true});},stop(){}};}};
  const links=[];
  overrides['react-native']=new Proxy({StyleSheet:{create:s=>s,absoluteFill:{position:'absolute',top:0,left:0,right:0,bottom:0}},useWindowDimensions:()=>env.dimensions,Keyboard:env.keyboard,Animated,Linking:{openURL(...args){links.push(args);return openURL(...args);}},
    AccessibilityInfo:{isReduceMotionEnabled:()=>Promise.resolve(true),sendAccessibilityEvent:(target,event)=>events.push([target._label,event]),addEventListener:(name,fn)=>{motionListeners[name]=fn;return{remove(){delete motionListeners[name];}};}},
    BackHandler:{addEventListener:(_name,fn)=>{env.back=fn;return{remove(){env.back=undefined;}};}}},{get:(o,k)=>o[k]??k});
  for(const name of ['avatar','menu','notifications','message','plus','mic','send','home','services','requests','profile'])overrides[`../assets/figma/chat-home/${name}.png`]='file:///'+path.join(root,'assets/figma/chat-home',name+'.png').replaceAll('\\','/');
  for(const name of ['emblem','wordmark','close','new-chat','recent','applications','chevron','history','more','settings'])overrides[`../assets/figma/chat-home-menu/${name}.png`]='file:///'+path.join(root,'assets/figma/chat-home-menu',name+'.png').replaceAll('\\','/');
  for(const name of ['back','new-chat','avatar','status','id-card','arrow','globe','profile-bell'])overrides[`./assets/figma/chat-conversation/${name}.png`]='file:///'+path.join(root,'assets/figma/chat-conversation',name+'.png').replaceAll('\\','/');
  overrides['expo-font']={useFonts:()=>fontError?[false,Error('font')]:[true,null]};
  const load=loader(rt,env,overrides);
  const component=sourceOnly ? ()=>load('components/OfficialSourceReference.tsx').OfficialSourceReference({icon:1}) : mode==='shared' ? props=>load('components/ChatHomeContent.tsx').ChatHomeContent({...props,mode:'standalone',authenticatedDisplayName:displayName}) : load('ChatHomePreviewScreen.tsx')[mode==='demo'?'ChatHomeDemoScreen':'ChatHomePreviewScreen'];
  const controls=new Map();
  let tree, exits=0, clears=0, viewportHeight=height-48, composerY=height-150;
  const scrolls=[];
  const draw=()=>{
    tree=rt.render(()=>expand(component({onExit})));
    for(const n of nodes(tree))if(n.props.ref){
      const control=controls.get(n.props.ref) || (n.type==='TextInput'?{clear(){clears++;},blur(){blurs++;nodes(tree,'TextInput')[0]?.props.onBlur?.();}}
        :n.type==='ScrollView'?{scrollTo(p){scrolls.push({...p,source:n.props.testID});},scrollToEnd(p){scrolls.push({end:true,...p,source:n.props.testID});}}
        :{measureInWindow(fn){fn(0,nodes(n,'TextInput').length&&!nodes(n,'ScrollView').length?composerY:24,width,nodes(n,'ScrollView').length?viewportHeight:84*fontScale);}});
      control._label=n.props.accessibilityLabel||n.type;control.focus=()=>focuses.push(control._label);controls.set(n.props.ref,control);
      n.props.ref.current=control;
    }
    rt.effects(); return tree;
  };
  function onExit(){exits++;exitCallback?.();}
  draw();nodes(tree,'Image').filter(n=>n.props.onLoad).forEach(n=>assetError?n.props.onError():n.props.onLoad());draw();
  return {rt,env,draw,scrolls,links,focuses,events,timings,motionListeners,get blurs(){return blurs;},get tree(){return tree;},get clears(){return clears;},get exits(){return exits;},
    set(value){nodes(tree,'TextInput')[0].props.onChangeText(value);draw();},
    measure(h,y){viewportHeight=h;composerY=y;nodes(tree,'View').find(n=>n.props.ref&&nodes(n,'ScrollView').length).props.onLayout();rt.flush();draw();},
    text(){return nodes(tree,'Text').flatMap(n=>[n.props.children].flat(Infinity)).filter(s=>typeof s==='string');},
    unmount(){rt.unmount();assert.equal(rt.frames.size,0);assert.equal(rt.timers.size,0);}};
}
function app() {
  const rt=runtime(),env=environment(),overrides={};
  for(const [name,asset] of [['WelcomeScreen','welcomeAssets'],['LoginScreen','loginAssets'],['RegisterScreen','registerAssets'],['ForgotPasswordScreen','forgotPasswordAssets'],['OnboardingMeetScreen','onboardingAssets'],['OnboardingDiscoverScreen','discoverAssets'],['OnboardingAskScreen','askAssets'],['OnboardingTrackScreen','trackAssets'],['OnboardingPayScreen','payAssets']])overrides['./'+name]={[name]:name,[asset]:{hero:1}};
  for(const name of ['AccountCreatedPreviewScreen','CheckEmailPreviewScreen','ResetPasswordPreviewScreen','PasswordUpdatedPreviewScreen','ChatHomePreviewScreen'])overrides['./'+name]={[name]:name};
  const {Startup}=loader(rt,env,overrides)('App.tsx');
  const draw=()=>{const t=rt.render(()=>Startup());rt.effects();return t;};
  let tree=draw();assert.notEqual(tree.type,'ChatHomePreviewScreen');nodes(tree,'Image').forEach(n=>n.props.onLoad?.());tree=draw();
  return{rt,env,draw,tree};
}
try {
  for(const dev of [false,true])for(const value of [undefined,'0','1','true']){
    flags(dev,value);const enabled=dev&&value==='1',a=app();assert.equal(a.tree.type,enabled?'ChatHomePreviewScreen':'WelcomeScreen');
    if(enabled)for(const [displayName,expected]of [[undefined,'Hello'],[null,'Hello'],['  ','Hello'],['  Citizen Example  ','Hello, Citizen Example'],['A very long trusted display name that must wrap','Hello, A very long trusted display name that must wrap']]){const g=fixture(400,772,1,{mode:'shared',displayName});assert(g.text().includes(expected));assert(nodes(g.tree,'Text').some(n=>n.props.accessibilityLabel===expected));g.unmount();}
  const f=fixture();if(enabled)assert(f.text().includes('Hello'));assert.equal(f.tree===null,!enabled);f.unmount();
    if(enabled){a.tree.props.onExit();assert.equal(a.draw().type,'WelcomeScreen');}a.rt.unmount();
  }
  for(const dev of [false,true])for(const value of [undefined,'0','1','true']){flags(dev,'0');process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO=value;const d=fixture(400,772,1,{mode:'demo'});assert.equal(d.tree===null,!(dev&&value==='1'));if(d.tree){assert(d.text().includes('Hello'));assert(d.text().includes('UI demo \u00b7 Credentials were not verified. AI and actions are disconnected.'));d.set('sample draft');d.env.visible=true;d.env.back();assert.equal(d.exits,0);d.env.back();d.draw();assert.equal(d.exits,1);assert.equal(nodes(d.tree,'TextInput')[0].props.value,'');}d.unmount();}
  for(const [i,name] of ['AccountCreatedPreviewScreen','CheckEmailPreviewScreen','ResetPasswordPreviewScreen','PasswordUpdatedPreviewScreen'].entries()){
    flags(true,'1');process.env[keys[i+1]]='1';const a=app();assert.equal(a.tree.type,name);a.rt.unmount();
  }
  flags(true,'1');process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO='1';{const a=app();assert.equal(a.tree.type,'WelcomeScreen','existing connected demo wins combined flags');a.rt.unmount();}
  flags(true,'1');
  const f=fixture();assert.equal(nodes(f.tree,'TextInput')[0].props.value,'');
  assert(f.text().includes('Development preview · Local mock chat works. Authentication, AI and backend are disconnected; service actions are unavailable.'));
  assert(!f.text().includes('Development preview · Authentication and AI are disconnected; actions are unavailable.'));
  f.set('Local sample draft');assert.equal(nodes(f.tree,'TextInput')[0].props.value,'Local sample draft');
  assert(!f.text().includes('Exit preview'));
  for(const n of nodes(f.tree,'Pressable')){if(['Open menu','How do I renew my NIC?','What do I need for a passport?','Pay my vehicle licence','Send local message','Simulate next response error'].includes(n.props.accessibilityLabel)){assert(!n.props.disabled);assert.equal(typeof n.props.onPress,'function');}else{assert(n.props.disabled);assert(n.props.accessibilityState.disabled);assert(!n.props.onPress);}}
  assert.equal(nodes(f.tree,'Pressable').length,12);
  assert.equal(nodes(f.tree,'Pressable').filter(n=>n.props.accessibilityState.selected).length,1);
  f.env.visible=true;f.env.back();f.draw();assert.equal(f.exits,0);assert.equal(nodes(f.tree,'TextInput')[0].props.value,'Local sample draft');
  f.env.back();f.draw();assert.equal(f.exits,1);assert.equal(nodes(f.tree,'TextInput')[0].props.value,'');assert(f.clears>0);
  f.env.back();assert.equal(f.exits,1,'duplicate Back cannot exit twice');f.unmount();
  const u=fixture();u.set('discard on unmount');u.unmount();assert(u.clears>0);
  for(const options of [{fontError:true},{assetError:true}]){const e=fixture(400,772,1,options);assert(e.text().includes('Unable to load Chat Home preview assets. Reload the preview.'));assert(nodes(e.tree,'Text').some(n=>n.props.accessibilityRole==='alert'));e.env.back();assert.equal(e.exits,1);e.unmount();}
  let layouts=0;
  for(const [w,h]of [[320,568],[360,640],[360,800],[412,915],[600,960],[400,772]])for(const scale of [1,2,3]){
    const f=fixture(w,h,scale);assert.equal(f.scrolls.length,0,'no initial forced scroll');
    const sv=nodes(f.tree,'ScrollView')[0];assert(!('scrollEnabled'in sv.props));assert.equal(sv.props.automaticallyAdjustKeyboardInsets,false);
    for(const n of nodes(f.tree,'Pressable')){const s=flat(n.props.style);assert((s.height??s.minHeight)>=48);assert((s.width??s.minWidth)>=48);}
    const images=nodes(f.tree,'Image');for(const n of images){const s=flat(n.props.style);assert(s.width>0&&s.height>0);}
    const crop=flat(images.find(n=>n.props.resizeMode==='stretch').props.style);assert.equal(crop.width,62*1.1084);assert.equal(crop.height,62*1.0412);assert.equal(crop.left,-62*.0438);assert.equal(crop.top,-62*.0507);
    nodes(f.tree,'TextInput')[0].props.onFocus();f.env.visible=true;f.env.listeners.keyboardDidShow({endCoordinates:{screenY:300}});f.rt.flush();f.draw();assert(f.scrolls.length>0);
    f.measure(276,500);assert.equal(flat(nodes(f.tree,'ScrollView')[0].props.contentContainerStyle).paddingBottom,0,'resized window has no duplicated overlap');
    f.env.visible=false;f.env.listeners.keyboardDidHide();f.draw();f.unmount();layouts++;
  }
  const launcher=read('scripts/preview-chat-home.cjs');
  for(const args of [[],['--port','8096']]){
    let launch,exit;const parent=Object.fromEntries(keys.map(k=>[k,'1']));
    vm.runInNewContext(launcher,{__dirname:path.join(root,'scripts'),require:n=>n==='node:path'?path:{spawnSync:(exe,args,options)=>(launch={args,options},{status:7})},process:{argv:['node','preview',...args],execPath:process.execPath,env:parent,exit:s=>exit=s},console:{log(){},error(){}}});
    assert.deepEqual(Array.from(launch.args).slice(1),['start','--go','--lan',...args]);assert.equal(exit,7);assert.equal(launch.options.env[keys[0]],'1');keys.slice(1).forEach(k=>assert.equal(launch.options.env[k],'0'));assert(keys.every(k=>parent[k]==='1'));
  }
  for(const args of [['--port','0'],['--port','65536'],['--port','x'],['--dev']])assert.throws(()=>vm.runInNewContext(launcher,{require:n=>n==='node:path'?path:{},process:{argv:['node','preview',...args]},console}),/Optional supported argument/);
  const assets=path.join(root,'assets/figma/chat-home');
  for(const [file,expected]of Object.entries(JSON.parse(read('assets/figma/chat-home/sha256.json'))))assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join(assets,file))).digest('hex'),expected,'approved original/raster/font hash: '+file);
  for(const file of fs.readdirSync(assets).filter(n=>n.endsWith('.png'))){const bytes=fs.readFileSync(path.join(assets,file));assert.equal(bytes.subarray(0,8).toString('hex'),'89504e470d0a1a0a');assert(bytes.readUInt32BE(16)>0&&bytes.readUInt32BE(20)>0);}
  assert(read('assets/figma/chat-home/fonts/OFL.txt').includes('SIL OPEN FONT LICENSE'));
  if(process.argv.includes('--export')){
    const dir=path.resolve(root,process.argv[process.argv.indexOf('--export')+1]||'dist/android'),files=[];
    function walk(d){for(const e of fs.readdirSync(d,{withFileTypes:true})){const f=path.join(d,e.name);e.isDirectory()?walk(f):files.push(f);}}walk(dir);
    const bundles=files.filter(f=>/\.(hbc|js)$/.test(f));assert(bundles.length);
    for(const file of bundles){const b=fs.readFileSync(file);for(const marker of ['ChatHomePreviewScreen','Message draft','Authentication and AI are disconnected; actions are unavailable.','ChatHomeDemoScreen','Credentials were not verified. AI and actions are disconnected.','Fraunces_700Bold','Sample chat history','ChatHomeMenu','Unavailable: no login occurred in this preview.','UI demo only · Returns to Login.'])for(const encoding of ['utf8','utf16le'])assert(!b.includes(Buffer.from(marker,encoding)),marker+' in production');}
    const hash=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
    const artifactHashes=new Set(files.map(hash));for(const file of ['avatar.png','fonts/Fraunces-Bold.ttf'])assert(!artifactHashes.has(hash(path.join(assets,file))),file+' in production');
    console.log('PASS Chat Home production module/copy/avatar/font exclusion:',dir);
  }
  if(process.argv.includes('--visual') || process.argv.includes('--menu-visual') || process.argv.includes('--conversation-visual')){
    const out=path.join(root,'.cache/chat-home-review');fs.mkdirSync(out,{recursive:true});
    const esc=s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;');
    const unitless=new Set(['flexGrow','flexShrink','opacity','zIndex','fontWeight']);
    function css(style,scale){return Object.entries(flat(style)).map(([k,v])=>{if(v===undefined)return'';if(k==='opacity'&&typeof v==='object')v=v.value;if(k==='flex')return`flex:${v} 1 0%;`;if(k==='paddingHorizontal')return`padding-left:${v}px;padding-right:${v}px;`;if(k==='paddingVertical')return`padding-top:${v}px;padding-bottom:${v}px;`;const key=k.replace(/[A-Z]/g,c=>'-'+c.toLowerCase());return`${key}:${typeof v==='number'&&!unitless.has(k)?v*(['fontSize','lineHeight'].includes(k)?scale:1)+'px':v};`;}).join('');}
    let reviewHeight=0;
    function html(t,scale){if(t===null||t===undefined||typeof t==='boolean')return'';if(typeof t!=='object')return esc(t);const p=t.props||{},children=[p.children].flat(Infinity).map(n=>html(n,scale)).join('');
      if(!t.type || typeof t.type==='symbol')return children;
      if(t.type==='StatusBar')return'';
      if(t.type==='Image')return`<img src="${esc(p.source)}" style="${css(p.style,scale)};object-fit:${p.resizeMode==='stretch'?'fill':'contain'}">`;
      if(t.type==='TextInput')return`<textarea placeholder="${esc(p.placeholder)}" style="${css(p.style,scale)}">${esc(p.value||'')}</textarea>`;
      const base=t.type==='Text'?'display:block;flex-shrink:0;white-space:pre-wrap;':'display:flex;position:relative;flex-direction:column;flex-shrink:0;min-width:0;min-height:0;border-style:solid;border-width:0;';
      const id=p.testID?` data-testid="${esc(p.testID)}"`:'';
      if(t.type==='ScrollView')return`<div${id} style="${base}${css(p.style,scale)}${p.testID?'':`height:${reviewHeight-48}px;`}min-height:0;overflow:${p.scrollEnabled===false?'hidden':'auto'}"><div style="display:flex;flex-direction:column;flex-shrink:0;${css(p.contentContainerStyle,scale)}">${children}</div></div>`;
      return`<div${id} style="${base}${css(p.style,scale)}">${children}</div>`;
    }
    for(const [w,h,scale]of [[400,772,1],[320,568,1],[400,772,2],[600,960,1],...(process.argv.includes('--menu-visual')?[[320,240,3]]:[])]){
      reviewHeight=h;const f=fixture(w,h,scale);f.env.insets={top:24,bottom:24,left:0,right:0};f.draw();
      if(process.argv.includes('--menu-visual')){nodes(f.tree,'Pressable').find(n=>n.props.accessibilityLabel==='Open menu').props.onPress();f.draw();}
      if(process.argv.includes('--conversation-visual')){
        nodes(f.tree,'Pressable').find(n=>n.props.accessibilityLabel==='How do I renew my NIC?').props.onPress();f.draw();
        const pending=[...f.rt.timers.values()];f.rt.timers.clear();pending.forEach(fn=>fn());f.draw();
      }
      const font='file:///'+path.join(assets,'fonts/Fraunces-Bold.ttf').replaceAll('\\','/');
      const inter='file:///'+path.join(root,'node_modules/@expo-google-fonts/inter').replaceAll('\\','/');
      const faces=[['Inter_400Regular','400Regular/Inter_400Regular.ttf'],['Inter_500Medium','500Medium/Inter_500Medium.ttf'],['Inter_600SemiBold','600SemiBold/Inter_600SemiBold.ttf'],['Inter_700Bold','700Bold/Inter_700Bold.ttf']].map(([name,file])=>`@font-face{font-family:${name};src:url('${inter}/${file}')}`).join('');
      const measureDrawer=`<script>document.fonts.ready.then(()=>{const q=id=>document.querySelector('[data-testid="'+id+'"]'),shell=q('drawer-shell');if(!shell)return;const header=q('drawer-header'),footer=q('drawer-footer'),middle=q('drawer-middle'),content=shell.firstElementChild,s=getComputedStyle(content);const fallback=header.offsetHeight+footer.offsetHeight+parseFloat(s.paddingTop)+parseFloat(s.paddingBottom)+48>shell.clientHeight;if(fallback){content.style.height='auto';middle.style.flex='0 0 auto';middle.style.overflow='hidden';shell.style.overflow='auto';}shell.dataset.overflowMode=fallback?'whole':'middle';});</script>`;
      const measureChat=`<script>document.fonts.ready.then(()=>{const shell=document.querySelector('[data-testid="chat-shell"]');if(!shell)return;shell.style.height='${h-48}px';const list=document.querySelector('[data-testid="conversation-messages"]');if(!list)return;let fixed=0;for(const id of ['chat-header','chat-notices','chat-demo-controls','chat-composer','chat-navigation']){const element=document.querySelector('[data-testid="'+id+'"]');if(element)fixed+=element.offsetHeight;}shell.firstElementChild.style.minHeight=(fixed+96)+'px';});</script>`;
      fs.writeFileSync(path.join(out,`${w}x${h}-${scale}${process.argv.includes('--menu-visual')?'-menu':process.argv.includes('--conversation-visual')?'-conversation':''}.html`),`<!doctype html><meta charset="utf-8"><style>*{box-sizing:border-box}html,body{margin:0;width:100%;height:100%;display:flex;flex-direction:column}body{width:${w}px;height:${h}px}textarea{border:0;background:transparent;resize:none;outline:0}textarea::placeholder{color:#666}${faces}@font-face{font-family:Fraunces_700Bold;src:url('${font}')}</style>${html(f.tree,scale)}${measureDrawer}${measureChat}`);f.unmount();
    }
    console.log('HTML approximations generated:',out);
  }
  console.log(`PASS Chat Home: 8 mode/flag cases, standalone precedence, local draft/cleanup, keyboard-first Back, menu/suggestions/local Send with disabled unimplemented controls, loading errors, ${layouts} synthetic layout/keyboard cases, launcher isolation. No Android/device rendering.`);
}finally{global.__DEV__=savedDev;keys.forEach((k,i)=>saved[i]===undefined?delete process.env[k]:process.env[k]=saved[i]);}

module.exports={fixture,flags,nodes,flat};
