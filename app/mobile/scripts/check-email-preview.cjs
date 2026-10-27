// React/native mocks and artifact inspection; no physical-device or native-layout claim.
const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict'), vm = require('node:vm'), crypto = require('node:crypto');
const root = path.resolve(__dirname, '..');
global.__DEV__ = false;
const { runtime, environment, loader, nodes } = require('./check-register.cjs');
const flat = style => Object.assign({}, ...[style].flat(Infinity).filter(Boolean));
const source = fs.readFileSync(path.join(root, 'CheckEmailPreviewScreen.tsx'), 'utf8');
assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|Linking|console|setTimeout|setInterval)\b/.test(source));
assert(!/allowFontScaling=\{false\}|maxFontSizeMultiplier|numberOfLines|0:45|30 minutes|We sent/.test(source));
assert(!/CheckEmail|CHECK_EMAIL_PREVIEW/.test(fs.readFileSync(path.join(root, 'ForgotPasswordScreen.tsx'), 'utf8')), 'Forgot submission has no preview entry');
const previous = { dev: global.__DEV__, email: process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW, account: process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW };
function flags(dev, email, account = '0') {
  global.__DEV__ = dev;
  for (const [key, value] of [['EXPO_PUBLIC_CHECK_EMAIL_PREVIEW', email], ['EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW', account]]) {
    if (value === undefined) delete process.env[key]; else process.env[key] = value;
  }
}
function appFixture() {
  const rt = runtime(), env = environment(), overrides = {}; let imports = 0;
  for (const [file, asset] of [['WelcomeScreen','welcomeAssets'],['LoginScreen','loginAssets'],['RegisterScreen','registerAssets'],['ForgotPasswordScreen','forgotPasswordAssets'],['OnboardingMeetScreen','onboardingAssets'],['OnboardingDiscoverScreen','discoverAssets'],['OnboardingAskScreen','askAssets'],['OnboardingTrackScreen','trackAssets'],['OnboardingPayScreen','payAssets']]) overrides['./'+file] = { [file]: file, [asset]: { hero: 1 } };
  overrides['./AccountCreatedPreviewScreen'] = { AccountCreatedPreviewScreen: 'AccountCreatedPreviewScreen' };
  overrides['./CheckEmailPreviewScreen'] = { get CheckEmailPreviewScreen() { imports++; return 'CheckEmailPreviewScreen'; } };
  const { Startup } = loader(rt, env, overrides)('App.tsx');
  const draw = () => { const tree = rt.render(() => Startup()); rt.effects(); return tree; };
  let tree = draw(); assert.notEqual(tree.type, 'CheckEmailPreviewScreen', 'startup resource gate remains'); nodes(tree,'Image').forEach(n=>n.props.onLoad?.()); tree = draw();
  return { rt, env, draw, tree, get imports() { return imports; } };
}
let guardCases = 0;
for (const dev of [false,true]) for (const flag of [undefined,'0','1','true']) {
  flags(dev, flag); const f = appFixture(), enabled = dev && flag === '1';
  assert.equal(f.tree.type, enabled ? 'CheckEmailPreviewScreen' : 'WelcomeScreen'); assert.equal(f.imports, enabled ? 1 : 0);
  const rt = runtime(), env = environment(); const { CheckEmailPreviewScreen } = loader(rt,env)('CheckEmailPreviewScreen.tsx');
  assert.equal(rt.render(()=>CheckEmailPreviewScreen({onBack(){},onDifferentNic(){},onExit(){}})) === null, !enabled, 'direct mount guard');
  f.rt.unmount(); rt.unmount(); guardCases++;
}
flags(true,'1','1'); const both = appFixture(); assert.equal(both.tree.type,'AccountCreatedPreviewScreen'); assert.equal(both.imports,0); both.tree.props.onExit(); assert.equal(both.draw().type,'WelcomeScreen'); both.rt.unmount();
for (const action of ['onBack','onDifferentNic','onExit','androidBack']) {
  flags(true,'1'); const f=appFixture();
  if(action==='androidBack') assert(f.env.back()); else f.tree.props[action]();
  let tree=f.draw(); assert.equal(tree.type,action==='onExit'?'WelcomeScreen':'ForgotPasswordScreen');
  if(action!=='onExit'){tree.props.onLogin();tree=f.draw();assert.equal(tree.type,'LoginScreen');tree.props.onBack();tree=f.draw();assert.equal(tree.type,'WelcomeScreen','standalone preview preserves initial Login origin');}
  f.rt.unmount();
}
let layouts=0;
for(const [width,height]of [[320,568],[360,640],[360,800],[412,915],[600,960],[400,772]])for(const fontScale of[1,2,3]){
  flags(true,'1');const rt=runtime(),env=environment(width,height,fontScale),{CheckEmailPreviewScreen}=loader(rt,env)('CheckEmailPreviewScreen.tsx');let tree,backs=0,others=0,exits=0;
  const draw=()=>{tree=rt.render(()=>CheckEmailPreviewScreen({onBack(){backs++;},onDifferentNic(){others++;},onExit(){exits++;}}));rt.effects();};draw();
  function measure(){
    nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).flex===1).props.onLayout({nativeEvent:{layout:{width:Math.min(480,width-32),height:height-80}}});
    nodes(tree,'Text').find(n=>n.props.onLayout).props.onLayout({nativeEvent:{layout:{height:18*fontScale*fontScale}}});
    nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).width===86).props.onLayout({nativeEvent:{layout:{height:47*fontScale}}});
    nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).paddingVertical===20).props.onLayout({nativeEvent:{layout:{height:383*fontScale}}});draw();
  }measure();const writes=rt.writes;for(let i=0;i<4;i++)measure();assert.equal(rt.writes,writes,'measured layout converges');
  const texts=nodes(tree,'Text').map(n=>n.props.children);assert(texts.includes('sample@example.test'));assert(texts.includes('Preview \u00b7 No email sent'));assert(texts.includes('No reset link exists or expires in this preview.'));
  assert.equal(nodes(tree,'TextInput').length,0);const disabled=nodes(tree,'Pressable').filter(n=>n.props.disabled);assert.equal(disabled.length,1);
  for(const n of disabled){assert(n.props.accessibilityState.disabled);assert(!n.props.onPress,'disabled actions have no handler');}
  for(const n of nodes(tree,'Pressable')){const s=flat(n.props.style);assert((s.minHeight??s.height)>=48);const targetWidth=s.minWidth??(s.width==='100%'?Math.min(480,width-32)-42:s.width??48);assert(targetWidth>=48);}
  const scroll=nodes(tree,'ScrollView')[0];assert.equal(scroll.props.showsVerticalScrollIndicator,false);assert(!('scrollEnabled'in scroll.props));
  assert(!nodes(tree,'Pressable').some(n=>n.props.accessibilityLabel==='Exit preview and return to Welcome'),'no visible Exit preview control');assert(!texts.includes('Exit preview'));assert(!nodes(tree,'Text').some(n=>n.props.onPress),'no text actions');
  const logo=nodes(tree,'View').find(n=>n.props.accessibilityRole==='image'),s=flat(logo.props.style);assert(Math.abs(s.height/s.width-151.531/150)<1e-5);assert(s.width/2+4<=Math.min(480,width-32)/2-86-9+0.01);
  const back=flat(nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel==='Back to Forgot Password').props.style);assert(back.left>=0&&back.top>=0&&back.zIndex>=2);
  const card=flat(nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).paddingVertical===20).props.style);assert(!('height'in card)&&!('minHeight'in card)&&!('maxHeight'in card),'card stays intrinsic');
  assert.equal(flat(disabled[0].props.style).backgroundColor,'#8b1730');assert(texts.includes('Unavailable in this preview'));
  assert(nodes(tree,'Text').some(n=>n.props.children==='Didn\u2019t get it? Resend'&&n.props.accessibilityState.disabled&&!n.props.onPress));
  assert(texts.includes('Resend unavailable in this preview'));
  const badge=nodes(tree,'View').find(n=>flat(n.props.style).backgroundColor==='#f1f7fc');assert.equal(flat(badge.props.style).width,72);assert.equal(flat(nodes(badge,'Image')[0].props.style).width,34);
  nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel==='Back to Forgot Password').props.onPress();
  nodes(tree,'Pressable').find(n=>n.props.accessibilityRole==='link').props.onPress();
  assert.equal(backs,1);assert.equal(others,1);assert.equal(exits,0);rt.unmount();layouts++;
}
for(const [args,status]of [[[],0],[['--port','8082'],7],[[],null]]){
  let launch,exit;const parent={EXPO_PUBLIC_CHECK_EMAIL_PREVIEW:'0',EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW:'1',KEEP:'same'};
  vm.runInNewContext(fs.readFileSync(path.join(root,'scripts/preview-check-email.cjs'),'utf8'),{__dirname:path.join(root,'scripts'),require:n=>n==='node:path'?path:n==='node:child_process'?{spawnSync:(exe,a,options)=>(launch={exe,args:a,options},{status})}:require(n),process:{argv:['node','preview',...args],execPath:process.execPath,env:parent,exit:s=>{exit=s;}},console:{log(){},error(){}}});
  assert.deepEqual(Array.from(launch.args).slice(1),['start','--go','--lan',...args]);assert.equal(launch.options.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW,'1');assert.equal(launch.options.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW,'0');assert.equal(parent.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW,'0');assert.equal(parent.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW,'1');assert.equal(launch.options.env.KEEP,'same');assert.equal(exit,status??1);
}
const svg=fs.readFileSync(path.join(root,'assets/figma/check-email/mail.svg'));assert.equal(crypto.createHash('sha256').update(svg).digest('hex'),'cdbae6793b123dbb6e5e5f7bc111d84df0fd7eaa03563a07d5d030ae96620bcb');const png=fs.readFileSync(path.join(root,'assets/figma/check-email/mail.png'));assert.equal(png.subarray(0,8).toString('hex'),'89504e470d0a1a0a');assert.equal(png.readUInt32BE(16),136);assert.equal(png.readUInt32BE(20),136);
if(process.argv.includes('--export')){
  let files=[];function walk(d){for(const e of fs.readdirSync(d,{withFileTypes:true})){const f=path.join(d,e.name);if(e.isDirectory())walk(f);else files.push(f);}}walk(path.join(root,'dist/android'));
  const iconHash=crypto.createHash('sha256').update(png).digest('hex');assert(!files.some(f=>crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex')===iconHash),'exclusive mail asset excluded from production');
  const bundles=files.filter(f=>/\.(hbc|js)$/.test(f));assert(bundles.length);for(const f of bundles){const b=fs.readFileSync(f);for(const marker of ['sample@example.test','CheckEmailPreviewScreen','No email sent'])assert(!b.includes(Buffer.from(marker))&&!b.includes(Buffer.from(marker,'utf16le')),'preview marker in release');}
  console.log('PASS current production artifact: Check Email module/sample copy/mail asset excluded.');
}
flags(previous.dev,previous.email,previous.account);
console.log(`PASS Check Email: ${guardCases} development/flag cases, Account Created precedence, direct guard, all exit routes, static sample/disabled actions/no side effects, launcher port/env/exit handling, ${layouts} synthetic layout/convergence cases. No native/device rendering.`);
