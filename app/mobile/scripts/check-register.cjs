// Node/TypeScript-only checks. Native layout, fonts and IME behaviour require a phone.
// Direct runs verify disconnected behavior in both Metro modes. Imported suites
// provide their own mode; no assertion or optional snapshot is skipped.
if (require.main === module && process.env.GOVAI_REGISTER_TEST_MODE === undefined) {
  const { spawnSync } = require('node:child_process');
  for (const mode of ['production', 'development']) {
    const env = { ...process.env, GOVAI_REGISTER_TEST_MODE: mode };
    for (const key of ['EXPO_PUBLIC_CONNECTED_UI_DEMO', 'EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW', 'EXPO_PUBLIC_CHECK_EMAIL_PREVIEW', 'EXPO_PUBLIC_RESET_PASSWORD_PREVIEW', 'EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW']) env[key] = '0';
    console.log('Register harness mode:', mode);
    const result = spawnSync(process.execPath, [__filename], { env, stdio: 'inherit' });
    if (result.error) throw result.error;
    if (result.status !== 0) process.exit(result.status ?? 1);
  }
  process.exit(0);
}
if (require.main === module) {
  if (!['production', 'development'].includes(process.env.GOVAI_REGISTER_TEST_MODE)) throw Error('Invalid Register test mode');
  global.__DEV__ = process.env.GOVAI_REGISTER_TEST_MODE === 'development';
} else if (typeof global.__DEV__ !== 'boolean') {
  global.__DEV__ = false;
}
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const ts = require('typescript');
const root = path.resolve(__dirname, '..');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const source = read('RegisterScreen.tsx');
assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|console)\b/.test(source), 'No requests, storage or logging');
assert(!/allowFontScaling=\{false\}|maxFontSizeMultiplier|numberOfLines/.test(source), 'No text scaling/truncation restrictions');
const names = ['fullName','nic','email','phone','password','confirmPassword'];
const placeholders = ['Full name as on your NIC','200012345678 or 991234567V','name@gmail.com','07X XXX XXXX','Create a password','Re-enter your password'];
const jsx = (type,props) => ({type,props});
const flat = style => Object.assign({}, ...[style].flat(Infinity).filter(Boolean));
function nodes(tree,type) { if (!tree || typeof tree !== 'object') return [];
  return [tree,...[tree.props?.children].flat(Infinity).flatMap(n => nodes(n))].filter(n => !type || n.type === type); }
function runtime() {
  let index=0,next=1,writes=0;
  const slots=[],pending=[],cleanups=[],frames=new Map(),timers=new Map();
  const react={
    useState(initial){const i=index++;if(!(i in slots))slots[i]=typeof initial==='function'?initial():initial;
      return[slots[i],value=>{const next=typeof value==='function'?value(slots[i]):value;if(!Object.is(next,slots[i])){slots[i]=next;writes++;}}];},
    useRef(initial){const i=index++;return slots[i]??(slots[i]={current:initial});},
    useEffect(effect,deps){const i=index++,old=slots[i];if(!old||!deps||deps.some((v,j)=>v!==old[j])){slots[i]=deps;pending.push(()=>{cleanups[i]?.();cleanups[i]=effect();});}},
  };
  return{react,frames,timers,get writes(){return writes;},render(fn){index=0;return fn();},effects(){pending.splice(0).forEach(f=>f());},
    raf(fn){const id=next++;frames.set(id,fn);return id;},cancel(id){frames.delete(id);},
    timeout(fn){const id=next++;timers.set(id,fn);return id;},clear(id){timers.delete(id);},
    flush(){let count=0;while(frames.size){const[id,fn]=frames.entries().next().value;frames.delete(id);fn();assert(++count<100,'measurement loop');}},
    unmount(){cleanups.forEach(f=>f?.());},
  };
}
function loader(rt,env,overrides={}) {
  const cache={};
  function load(file){if(cache[file])return cache[file];let input=read(file);if(file==='App.tsx')input+='\nexport { Startup };';
    const output=ts.transpileModule(input,{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX}}).outputText;
    const module={exports:{}};cache[file]=module.exports;
    const native=new Proxy({StyleSheet:{create:s=>s,absoluteFill:{}},useWindowDimensions:()=>env.dimensions,Keyboard:env.keyboard,
      BackHandler:{addEventListener:(_name,fn)=>{env.back=fn;return{remove(){env.back=undefined;}};}}},{get:(o,k)=>o[k]??k});
    new Function('require','module','exports','requestAnimationFrame','cancelAnimationFrame','setTimeout','clearTimeout','__DEV__',output)(name=>{
      if(name in overrides)return overrides[name];
      if(name==='react')return rt.react;if(name==='react/jsx-runtime')return{jsx,jsxs:jsx};if(name==='react-native')return native;
      if(name==='expo-status-bar')return{StatusBar:'StatusBar'};if(name==='expo-font')return{useFonts:()=>[true,null]};
      if(name.startsWith('@expo-google-fonts/'))return{};
      if(name==='react-native-safe-area-context')return{SafeAreaProvider:'SafeAreaProvider',useSafeAreaInsets:()=>env.insets};
      if(/\.(png|jpg|ttf)$/.test(name))return 1;
      if(name.startsWith('.')){const target=path.relative(root,path.resolve(root,path.dirname(file),name));return load(target+(fs.existsSync(path.join(root,target+'.ts'))?'.ts':'.tsx'));}
      throw Error('Unexpected dependency: '+name);
    },module,module.exports,rt.raf,rt.cancel,rt.timeout,rt.clear,global.__DEV__);return cache[file]=module.exports;
  }return load;
}
function environment(width=360,height=800,fontScale=1){const env={dimensions:{width,height,fontScale},insets:{top:24,bottom:24,left:0,right:0},visible:false,metrics:undefined,listeners:{},dismisses:0};
  env.keyboard={isVisible:()=>env.visible,metrics:()=>env.metrics,dismiss(){env.visible=false;env.metrics=undefined;env.dismisses++;},
    addListener(name,fn){env.listeners[name]=fn;return{remove(){delete env.listeners[name];}};}};return env;
}
function registerFixture(width=360,height=800,fontScale=1) {
  const rt=runtime(),env=environment(width,height,fontScale),component=loader(rt,env)('RegisterScreen.tsx');
  let tree,exits=0,viewportHeight=height-80;const controls={},clears=[],focuses=[],scrolls=[];
  const fields=()=>nodes(tree,'TextInput');
  function draw(){tree=rt.render(()=>component.RegisterScreen({onLogin(){exits++;}}));
    for(const n of nodes(tree))if(n.props.ref){let control;
      if(n.type==='TextInput'){const key=names[placeholders.indexOf(n.props.placeholder)];control=controls[key]??(controls[key]={focus(){focuses.push(key);fields().find(f=>f.props.placeholder===placeholders[names.indexOf(key)]).props.onFocus();},clear(){clears.push(key);}});}
      else if(n.type==='ScrollView')control={scrollTo:point=>scrolls.push(point)};
      else {const child=nodes(n,'ScrollView').length?undefined:nodes(n,'TextInput')[0],index=child?placeholders.indexOf(child.props.placeholder):-1;
        control={measureInWindow:fn=>child?fn(16,170+index*100,Math.min(480,width-32),72*fontScale):fn(16,46,Math.min(480,width-32),viewportHeight)};}
      if(typeof n.props.ref==='function')n.props.ref(control);else n.props.ref.current=control;
    }rt.effects();return tree;
  }
  draw();
  function set(key,value){fields()[names.indexOf(key)].props.onChangeText(value);draw();}
  function submit(){nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel?.startsWith('Sign up.')).props.onPress();draw();rt.flush();draw();}
  function layout(h=height-80,formHeight=850*fontScale){viewportHeight=h;
    nodes(tree,'View').find(n=>n.props.ref&&typeof n.props.ref!=='function'&&n.props.onLayout).props.onLayout({nativeEvent:{layout:{width:Math.min(480,width-32),height:h}}});
    nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).gap===14).props.onLayout({nativeEvent:{layout:{height:formHeight}}});
    nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).width===86).props.onLayout({nativeEvent:{layout:{height:39*fontScale+8}}});
    rt.flush();draw();
  }
  function toggle(label){nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel===label).props.onPress();draw();}
  function show(top){env.visible=true;env.metrics={screenY:top};env.listeners.keyboardDidShow({endCoordinates:env.metrics});rt.flush();draw();}
  function hide(){env.visible=false;env.metrics=undefined;env.listeners.keyboardDidHide();rt.flush();draw();}
  function text(){return nodes(tree,'Text').flatMap(n=>[n.props.children].flat(Infinity)).filter(v=>typeof v==='string');}
  function unmount(){for(const n of nodes(tree))if(typeof n.props.ref==='function')n.props.ref(null);rt.unmount();assert.equal(rt.frames.size,0);assert.equal(rt.timers.size,0);}
  return{rt,env,draw,set,submit,layout,toggle,show,hide,text,unmount,clears,focuses,scrolls,fields,get tree(){return tree;},get exits(){return exits;}};
}

// Requiredness, raw passwords, no invented identifier validation, optional consent.
const f=registerFixture();assert.equal(f.fields().length,6);assert(f.fields()[4].props.secureTextEntry);assert(f.fields()[5].props.secureTextEntry);
assert.equal(nodes(f.tree,'Pressable').find(n=>n.props.accessibilityRole==='checkbox').props.accessibilityState.checked,false);
const terms=nodes(f.tree,'Text').find(n=>n.props.accessibilityRole==='link');assert(terms.props.accessibilityState.disabled);assert(!terms.props.onPress);assert(f.text().includes('Terms of Service (unavailable)'));
f.submit();assert.equal(f.focuses.at(-1),'fullName');assert.equal(nodes(f.tree,'Text').filter(n=>n.props.accessibilityLiveRegion==='polite').length,7);
const valid={fullName:'සිංහල தமிழ் name',nic:'arbitrary identifier',email:'not-an-email',phone:'not-a-phone',password:' raw password ',confirmPassword:' raw password '};
for(const[key,value]of Object.entries(valid))f.set(key,value);
for(const key of names){f.set(key,key==='password'||key==='confirmPassword'?'':'   ');f.submit();assert.equal(f.focuses.at(-1),key,'first invalid '+key);f.set(key,valid[key]);}
f.set('confirmPassword','raw password');f.submit();assert.equal(f.focuses.at(-1),'confirmPassword');assert(f.text().includes('Passwords do not match.'));
f.set('confirmPassword',valid.confirmPassword);f.submit();assert(f.text().includes('Registration is not connected yet. No registration request was sent. No account was created.'));assert.equal(f.exits,0);
for(const[key,value]of Object.entries(valid))assert.equal(f.fields()[names.indexOf(key)].props.value,value,'value unchanged');
assert.equal(nodes(f.tree,'Pressable').find(n=>n.props.accessibilityRole==='checkbox').props.accessibilityState.checked,false,'unchecked consent did not block');
f.set('password',' ');f.set('confirmPassword',' ');f.submit();assert.equal(f.fields()[4].props.value,' ','raw nonempty spaces not transformed');
f.toggle('Show password');assert(!f.fields()[4].props.secureTextEntry);assert(f.fields()[5].props.secureTextEntry);
f.toggle('Show confirm password');assert(!f.fields()[5].props.secureTextEntry);f.toggle('Hide password');assert(f.fields()[4].props.secureTextEntry);assert(!f.fields()[5].props.secureTextEntry);
nodes(f.tree,'Pressable').find(n=>n.props.accessibilityRole==='checkbox').props.onPress();f.draw();assert(nodes(f.tree,'Pressable').find(n=>n.props.accessibilityRole==='checkbox').props.accessibilityState.checked);
nodes(f.tree,'Pressable').find(n=>n.props.accessibilityLabel==='Log in').props.onPress();f.draw();assert.equal(f.exits,1);assert(f.fields().every(n=>n.props.value===''));assert(f.fields()[4].props.secureTextEntry&&f.fields()[5].props.secureTextEntry);assert.equal(f.clears.length,6);
f.unmount();assert.equal(f.clears.length,12,'cleanup retains mounted inputs after callback ref detach');
const back=registerFixture();back.set('password','temporary');nodes(back.tree,'Pressable').find(n=>n.props.accessibilityLabel==='Back to Login').props.onPress();back.draw();assert.equal(back.exits,1);assert(back.fields().every(n=>n.props.value===''));back.unmount();

let layoutCases=0,keyboardCases=0;
for(const[width,height]of[[320,568],[360,640],[360,800],[412,915],[600,960],[400,772]])for(const fontScale of[1,2,3]){
 const g=registerFixture(width,height,fontScale);g.layout();
 const scroll=nodes(g.tree,'ScrollView')[0];assert.equal(scroll.props.showsVerticalScrollIndicator,false);assert.equal(scroll.props.alwaysBounceVertical,false);assert.equal(scroll.props.automaticallyAdjustKeyboardInsets,false);
 const contentWidth=Math.min(480,width-32),cardWidth=contentWidth-42;
 for(const n of nodes(g.tree,'Pressable')){const s=flat(n.props.style);assert((s.minHeight??s.height??0)>=48);assert((s.minWidth??s.width??cardWidth)>=48);}
 const images=nodes(g.tree,'Image');for(const n of images){const s=flat(n.props.style);if(s.width&&s.height)assert(s.width>0&&s.height>0);}
 const repeated=g.rt.writes;for(let i=0;i<4;i++)g.layout();assert.equal(g.rt.writes,repeated,'intrinsic layout converges');layoutCases++;
 for(const mode of['resize','overlay']){const top=height-260;
  if(mode==='resize')g.layout(top-46);else g.layout();g.fields()[5].props.onFocus();g.show(top);
  const viewportHeight=mode==='resize'?top-46:height-80;
  assert.equal(flat(nodes(g.tree,'ScrollView')[0].props.contentContainerStyle).paddingBottom,Math.max(0,46+viewportHeight-top));assert(g.scrolls.length>0,'field group reveal requested');
  g.submit();g.toggle('Show password');assert(!g.fields()[4].props.secureTextEntry);g.toggle('Hide password');g.hide();assert.equal(flat(nodes(g.tree,'ScrollView')[0].props.contentContainerStyle).paddingBottom,0);keyboardCases++;
 }g.unmount();
}

// Execute actual Startup route callbacks with screen renderers replaced by names.
const nav=runtime(),env=environment(),overrides={};
for(const[file,component,assets]of[['WelcomeScreen','WelcomeScreen','welcomeAssets'],['LoginScreen','LoginScreen','loginAssets'],['RegisterScreen','RegisterScreen','registerAssets'],['ForgotPasswordScreen','ForgotPasswordScreen','forgotPasswordAssets'],['OnboardingMeetScreen','OnboardingMeetScreen','onboardingAssets'],['OnboardingDiscoverScreen','OnboardingDiscoverScreen','discoverAssets'],['OnboardingAskScreen','OnboardingAskScreen','askAssets'],['OnboardingTrackScreen','OnboardingTrackScreen','trackAssets'],['OnboardingPayScreen','OnboardingPayScreen','payAssets']])overrides['./'+file]={[component]:component,[assets]:{hero:1}};
const app=loader(nav,env,overrides)('App.tsx');const draw=()=>{const t=nav.render(()=>app.Startup());nav.effects();return t;};
let tree=draw();nodes(tree,'Image').forEach(n=>n.props.onLoad?.());tree=draw();
for(const origin of['WelcomeScreen','OnboardingMeetScreen','OnboardingDiscoverScreen','OnboardingAskScreen','OnboardingTrackScreen','OnboardingPayScreen']){
 assert.equal(tree.type,origin);tree.props.onLogin();tree=draw();assert.equal(tree.type,'LoginScreen');
 tree.props.onForgotPassword();tree=draw();assert.equal(tree.type,'ForgotPasswordScreen');
 env.visible=true;assert(env.back());tree=draw();assert.equal(tree.type,'ForgotPasswordScreen');assert(!env.visible,'Forgot Back dismisses keyboard first');
 env.back();tree=draw();assert.equal(tree.type,'LoginScreen');tree.props.onForgotPassword();tree=draw();tree.props.onLogin();tree=draw();assert.equal(tree.type,'LoginScreen');
 tree.props.onBack();tree=draw();assert.equal(tree.type,origin,'Forgot keeps Login origin');tree.props.onLogin();tree=draw();
 tree.props.onSignUp();tree=draw();assert.equal(tree.type,'RegisterScreen');
 env.visible=true;assert(env.back());tree=draw();assert.equal(tree.type,'RegisterScreen','first Back only dismisses keyboard');assert(!env.visible);
 env.back();tree=draw();assert.equal(tree.type,'LoginScreen');tree.props.onSignUp();tree=draw();tree.props.onLogin();tree=draw();assert.equal(tree.type,'LoginScreen');tree.props.onBack();tree=draw();assert.equal(tree.type,origin);
 if(origin==='WelcomeScreen')tree.props.onGetStarted();else if(origin!=='OnboardingPayScreen')tree.props.onNext();tree=draw();
}nav.unmount();

// New source SVGs must exactly match downloads; derived PNGs must be nonempty.
const hash=file=>crypto.createHash('sha256').update(fs.readFileSync(path.join(root,file))).digest('hex');
const expectedHashes={
  "assets/figma/register/back.svg": "c04441f79943ac60594b0b0a580d0bd68349b6a9c29a52de08adb0e11520a895",
  "assets/figma/register/user.svg": "17bdad198a824a61d5f03e50d60db502f52c1786c72a3cc31808c17da50d629e",
  "assets/figma/register/identity.svg": "c183b8009f2fb25ffb5037e8535bd8e294c8b16a701b2459688f310ea5276d9a",
  "assets/figma/register/mail.svg": "040157b88f3b858663cad1153e298a233a50acbb4337b51a56f79fafeb1bd292",
  "assets/figma/register/phone.svg": "303e0a4337877c6d130c83ad4a3cf3ee4088b6b7f9c289252268865a86a4c2b5",
  "assets/figma/register/lock.svg": "b4ce86570b5f14e47daf42d3bd577eaad448ab32e3885d35dcc6be4980ca1c1b",
  "assets/figma/register/eye-off.svg": "bc8a9006076df5f325c1a12b8249405b0047170833ce6383a9aef88b01143ca0",
  "assets/figma/register/next.svg": "20d1ddf067409498d44bd0cea7ee939b3b539cc8a369e72cb535dfcd70fc2369",
  "assets/figma/login/top-ribbon.png": "9a73f4e9db45c52d6ce533623462fff3deda60a0164fce9ffb25b8aee1d0c4dc",
  "assets/figma/login/skyline.png": "869d971b57f896f9da92682d96fd71e6a83618db1bdf2a2b6243a53dc75cff10",
  "assets/figma/login/bottom-ribbon.png": "b7798b7f54574c53b9512a13c76c3223f1b9a10570a2f0284b82e7ae025d1bfe",
  "assets/figma/login/logo.png": "a46897acd200eb92e91654c638af69498a7698ce39d320a71387fcecf6786b48"
};
for(const[file,expected]of Object.entries(expectedHashes))assert.equal(hash(file),expected,file);
for(const name of['back','user','identity','mail','phone','lock','eye-off','next']){
 assert(fs.statSync(path.join(root,`assets/figma/register/${name}.png`)).size>100);
}
const manifest=JSON.parse(read('package.json'));assert.equal(manifest.dependencies.expo,'~57.0.31');assert(!manifest.dependencies['@react-native-async-storage/async-storage']);
console.log(`PASS: six-field rules/raw passwords, optional consent/disabled Terms, no requests/storage/logging, both password toggles, explicit exit and unmount clearing; ${layoutCases} layout/convergence and ${keyboardCases} keyboard cases; all six Login origins/Register Back routes; 12 original asset hashes. Calculations/mocked native APIs only; no device rendering.`);
module.exports = { runtime, environment, loader, nodes };
