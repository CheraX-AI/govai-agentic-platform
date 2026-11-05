function presentation(tree) {
  while (tree && typeof tree.type === 'function' && ['AccountCreatedContent','CheckEmailContent','ContentBody'].includes(tree.type.name)) tree = tree.type(tree.props);
  return tree;
}
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const ts = require('typescript');
const root = path.resolve(__dirname, '..');
const read = file => fs.readFileSync(path.join(root,file),'utf8');
const jsx = (type,props) => ({type,props});
const flat = style => Object.assign({},...[style].flat(Infinity).filter(Boolean));
function nodes(tree,type){if(!tree||typeof tree!=='object')return[];return[tree,...[tree.props?.children].flat(Infinity).flatMap(n=>nodes(n))].filter(n=>!type||n.type===type);}
function runtime(){let index=0,writes=0;const slots=[],pending=[],cleanups=[];return{
 react:{useState(initial){const i=index++;if(!(i in slots))slots[i]=typeof initial==='function'?initial():initial;return[slots[i],v=>{const n=typeof v==='function'?v(slots[i]):v;if(!Object.is(n,slots[i])){slots[i]=n;writes++;}}];},
 useRef(initial){const i=index++;return slots[i]??(slots[i]={current:initial});},
 useEffect(fn,deps){const i=index++,old=slots[i];if(!old||deps.some((v,j)=>v!==old[j])){slots[i]=deps;pending.push(()=>{cleanups[i]?.();cleanups[i]=fn();});}}},
 render(fn){index=0;const tree=fn();pending.splice(0).forEach(f=>f());return tree;},unmount(){cleanups.forEach(f=>f?.());},get writes(){return writes;},
};}
function loader(rt,env,overrides={}){const cache={};return function load(file){if(cache[file])return cache[file];let source=read(file);if(file==='App.tsx')source+='\nexport { Startup };';
 const output=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX}}).outputText;const module={exports:{}};cache[file]=module.exports;
 const native=new Proxy({StyleSheet:{create:s=>s,absoluteFill:{}},useWindowDimensions:()=>env.dimensions,
 Keyboard:{isVisible:()=>false,dismiss(){}},BackHandler:{addEventListener:(_name,fn)=>{env.back=fn;return{remove(){env.back=undefined;}};}}},{get:(o,k)=>o[k]??k});
 new Function('require','module','exports','__DEV__','process',output)(name=>{
  if(name in overrides){if(name==='./pages/AccountCreatedPreviewScreen')env.previewImports++;return overrides[name];}
  if(name==='react')return rt.react;if(name==='react/jsx-runtime')return{jsx,jsxs:jsx};if(name==='react-native')return native;
  if(name==='expo-font')return{useFonts:()=>[true,null]};if(name==='expo-status-bar')return{StatusBar:'StatusBar'};
  if(name.startsWith('@expo-google-fonts/'))return{};
  if(name==='react-native-safe-area-context')return{SafeAreaProvider:'SafeAreaProvider',useSafeAreaInsets:()=>({top:24,bottom:24,left:0,right:0})};
  if(/\.(png|jpg|ttf)$/.test(name))return 1;
  if(name.startsWith('.')){const target=path.relative(root,path.resolve(root,path.dirname(file),name));return load(target+(fs.existsSync(path.join(root,target+'.ts'))?'.ts':'.tsx'));}
  throw Error('Unexpected dependency '+name);
 },module,module.exports,env.dev,{env:{EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW:env.flag}});return cache[file]=module.exports;
};}
const previewSource=read('pages/AccountCreatedPreviewScreen.tsx') + read('components/AccountCreatedContent.tsx');
assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|console)\b/.test(previewSource));
assert(!/allowFontScaling=\{false\}|maxFontSizeMultiplier|numberOfLines/.test(previewSource));
assert(!/AccountCreated|ACCOUNT_CREATED_PREVIEW/.test(read('pages/RegisterScreen.tsx')),'Register has no preview entry');
const overrides={};for(const[file,name,assets]of[
 ['WelcomeScreen','WelcomeScreen','welcomeAssets'],['LoginScreen','LoginScreen','loginAssets'],['RegisterScreen','RegisterScreen','registerAssets'],
 ['OnboardingMeetScreen','OnboardingMeetScreen','onboardingAssets'],['OnboardingDiscoverScreen','OnboardingDiscoverScreen','discoverAssets'],['OnboardingAskScreen','OnboardingAskScreen','askAssets'],['OnboardingTrackScreen','OnboardingTrackScreen','trackAssets'],['OnboardingPayScreen','OnboardingPayScreen','payAssets'],
])overrides['./pages/'+file]={[name]:name,[assets]:{hero:1}};
overrides['./pages/AccountCreatedPreviewScreen']={AccountCreatedPreviewScreen:'AccountCreatedPreviewScreen'};
let guardCases=0;
for(const dev of[false,true])for(const flag of[undefined,'0','1','true']){
 const rt=runtime(),env={dev,flag,dimensions:{width:400,height:772,fontScale:1},previewImports:0};
 const {Startup}=loader(rt,env,overrides)('App.tsx');const draw=()=>rt.render(()=>Startup());let tree=draw();
 assert.notEqual(tree.type,'AccountCreatedPreviewScreen','startup resources still gate initial route');nodes(tree,'Image').forEach(n=>n.props.onLoad?.());tree=draw();
 const enabled=dev&&flag==='1';assert.equal(tree.type,enabled?'AccountCreatedPreviewScreen':'WelcomeScreen');assert.equal(env.previewImports,enabled?1:0,'no production preview require');
 if(enabled){assert(env.back());tree=draw();assert.equal(tree.type,'WelcomeScreen','Android Back exits to Welcome');}
 tree.props.onLogin();tree=draw();tree.props.onSignUp();tree=draw();assert.equal(tree.type,'RegisterScreen','normal navigation preserved');tree.props.onLogin();tree=draw();assert.equal(tree.type,'LoginScreen','Register return never opens preview');rt.unmount();
 const direct=runtime(),component=loader(direct,env,overrides)('pages/AccountCreatedPreviewScreen.tsx');
 const directTree=direct.render(()=>presentation(component.AccountCreatedPreviewScreen({onExit(){}})));assert.equal(directTree===null,!enabled,'direct-mount defence');guardCases++;
}
// Separate on-screen exit test, since Back already closed the first instance.
{
 const rt=runtime(),env={dev:true,flag:'1',dimensions:{width:400,height:772,fontScale:1},previewImports:0};
 const{Startup}=loader(rt,env,overrides)('App.tsx');const draw=()=>rt.render(()=>Startup());let t=draw();nodes(t,'Image').forEach(n=>n.props.onLoad?.());t=draw();t.props.onExit();t=draw();assert.equal(t.type,'WelcomeScreen');rt.unmount();
}
let layouts=0;
for(const[width,height]of[[320,568],[360,640],[360,800],[412,915],[600,960],[400,772]])for(const fontScale of[1,2,3]){
 const rt=runtime(),env={dev:true,flag:'1',dimensions:{width,height,fontScale},previewImports:0};
 const{AccountCreatedPreviewScreen}=loader(rt,env,overrides)('pages/AccountCreatedPreviewScreen.tsx');let exits=0;const draw=()=>rt.render(()=>presentation(AccountCreatedPreviewScreen({onExit(){exits++;}})));let tree=draw();
 function measure(){
  nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).flex===1).props.onLayout({nativeEvent:{layout:{width:Math.min(480,width-32),height:height-80}}});
  nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).borderColor==='#f5a915').props.onLayout({nativeEvent:{layout:{height:72*fontScale*fontScale}}});
  nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).width===86).props.onLayout({nativeEvent:{layout:{height:39*fontScale+8}}});
  nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).paddingVertical===24).props.onLayout({nativeEvent:{layout:{height:346*fontScale}}});tree=draw();
 }
 measure();const writes=rt.writes;for(let i=0;i<4;i++)measure();assert.equal(rt.writes,writes,'measurements converge without feedback');
 const scroll=nodes(tree,'ScrollView')[0];assert.equal(scroll.props.showsVerticalScrollIndicator,false);assert.equal(scroll.props.alwaysBounceVertical,false);
 assert.equal(nodes(tree,'TextInput').length,0);
 assert(nodes(tree,'Text').some(n=>n.props.children==='Development preview — sample content. No account has been created.'));
 assert(nodes(tree,'Text').some(n=>n.props.children==='Welcome to GovAI, Rathishan. You can verify your identity later from Profile to unlock every service.'));
 const buttons=nodes(tree,'Pressable');assert.equal(buttons.length,2);const proceed=buttons.find(n=>n.props.disabled);assert(proceed.props.accessibilityState.disabled);assert(!proceed.props.onPress);assert(flat(proceed.props.style).minHeight>=48);
 const exit=buttons.find(n=>!n.props.disabled);assert(flat(exit.props.style).minWidth>=48&&flat(exit.props.style).minHeight>=48);exit.props.onPress();assert.equal(exits,1);
 if(72*fontScale*fontScale>(height-80)/2)assert(nodes(scroll,'Pressable').includes(exit),'large banner scrolls instead of consuming all content height');
 const badge=nodes(tree,'View').find(n=>flat(n.props.style).backgroundColor==='#f2f8f4');assert.equal(flat(badge.props.style).width,72);assert.equal(flat(nodes(badge,'Image')[0].props.style).width,34);
 const logo=nodes(tree,'View').find(n=>n.props.accessibilityRole==='image');const ls=flat(logo.props.style);assert(ls.width>0&&ls.width<=150);assert(Math.abs(ls.height/ls.width-151.531/150)<.00001);
 const img=flat(nodes(logo,'Image')[0].props.style);assert(Math.abs(img.width/ls.width-1.5417)<.00001);assert(Math.abs(img.height/ls.height-2.7132)<.00001);layouts++;
}
// Exercise the command without starting Expo or a server.
let launched,exitStatus;const parentEnv={EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW:'0',KEEP:'unchanged'};
vm.runInNewContext(read('scripts/preview-account-created.cjs'),{
 __dirname:path.join(root,'scripts'),require:name=>name==='node:path'?path:name==='node:child_process'?{spawnSync:(exe,args,options)=>(launched={exe,args,options},{status:0})}:(()=>{throw Error(name);})(),
 process:{execPath:process.execPath,env:parentEnv,exit:code=>{exitStatus=code;}},console:{log(){},error(){}},
});
assert.equal(exitStatus,0);assert.deepEqual(Array.from(launched.args).slice(1),['start','--go','--lan']);assert(launched.args[0].endsWith('scripts'+path.sep+'expo.cjs'));assert.equal(launched.options.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW,'1');assert.equal(parentEnv.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW,'0');assert.equal(launched.options.cwd,root);
assert(!launched.args.some(a=>/export|build|gradle|run:android/.test(a)),'launcher never builds an APK');

if(process.argv.includes('--export')){
 const hash=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');const iconHash=hash(path.join(root,'assets/figma/account-created/check-circle.png'));
 const exportRoot=path.resolve(root,process.argv[process.argv.indexOf('--export')+1]||'dist/android');let files=[];function walk(dir){for(const item of fs.readdirSync(dir,{withFileTypes:true})){const file=path.join(dir,item.name);if(item.isDirectory())walk(file);else files.push(file);}}walk(exportRoot);
 assert(!files.some(file=>hash(file)===iconHash),'preview-only icon excluded from production assets');
 const bundles=files.filter(f=>f.endsWith('.hbc')||f.endsWith('.js'));assert(bundles.length>0);
 for(const file of bundles){const data=fs.readFileSync(file);for(const marker of['Rathishan','AccountCreatedPreviewScreen','Development preview — sample content. No account has been created.']){
  assert(!data.includes(Buffer.from(marker,'utf8'))&&!data.includes(Buffer.from(marker,'utf16le')),'preview marker in release bundle: '+marker);
 }}
 console.log('PASS production export inspection: no preview-only icon or sample-screen markers in exported bundle/assets.');
}
console.log(`PASS: ${guardCases} development/flag combinations (including production with flag=1), direct-mount guards, Exit/Android Back to Welcome, no Register entry, ${layouts} synthetic layout/convergence cases, disabled Continue, sample labelling, original logo proportions, launcher isolation. No native/device rendering.`);
