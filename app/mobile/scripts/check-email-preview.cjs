function presentation(tree) {
  while (tree && typeof tree.type === 'function' && ['AccountCreatedContent','CheckEmailContent','ContentBody'].includes(tree.type.name)) tree = tree.type(tree.props);
  return tree;
}
// React/native mocks and artifact inspection; no physical-device or native-layout claim.
const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict'), vm = require('node:vm'), crypto = require('node:crypto');
const root = path.resolve(__dirname, '..');
global.__DEV__ = false;
const { runtime, environment, loader, nodes } = require('./check-register.cjs');
const flat = style => Object.assign({}, ...[style].flat(Infinity).filter(Boolean));
const source = fs.readFileSync(path.join(root, 'pages/CheckEmailPreviewScreen.tsx'), 'utf8') + fs.readFileSync(path.join(root, 'components/CheckEmailContent.tsx'), 'utf8');
assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|Linking|console|setTimeout|setInterval)\b/.test(source));
assert(!/allowFontScaling=\{false\}|maxFontSizeMultiplier|numberOfLines|0:45|30 minutes|We sent/.test(source));
assert(!/CheckEmail|CHECK_EMAIL_PREVIEW/.test(fs.readFileSync(path.join(root, 'pages/ForgotPasswordScreen.tsx'), 'utf8')), 'Forgot submission has no preview entry');
const previous = { dev: global.__DEV__, email: process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW, account: process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW };
function flags(dev, email, account = '0') {
  global.__DEV__ = dev;
  for (const [key, value] of [['EXPO_PUBLIC_CHECK_EMAIL_PREVIEW', email], ['EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW', account]]) {
    if (value === undefined) delete process.env[key]; else process.env[key] = value;
  }
}
function appFixture() {
  const rt = runtime(), env = environment(), overrides = {}; let imports = 0;
  for (const [file, asset] of [['WelcomeScreen','welcomeAssets'],['LoginScreen','loginAssets'],['RegisterScreen','registerAssets'],['ForgotPasswordScreen','forgotPasswordAssets'],['OnboardingMeetScreen','onboardingAssets'],['OnboardingDiscoverScreen','discoverAssets'],['OnboardingAskScreen','askAssets'],['OnboardingTrackScreen','trackAssets'],['OnboardingPayScreen','payAssets']]) overrides['./pages/'+file] = { [file]: file, [asset]: { hero: 1 } };
  overrides['./pages/AccountCreatedPreviewScreen'] = { AccountCreatedPreviewScreen: 'AccountCreatedPreviewScreen' };
  overrides['./pages/CheckEmailPreviewScreen'] = { get CheckEmailPreviewScreen() { imports++; return 'CheckEmailPreviewScreen'; } };
  const { Startup } = loader(rt, env, overrides)('App.tsx');
  const draw = () => { const tree = rt.render(() => Startup()); rt.effects(); return tree; };
  let tree = draw(); assert.notEqual(tree.type, 'CheckEmailPreviewScreen', 'startup resource gate remains'); nodes(tree,'Image').forEach(n=>n.props.onLoad?.()); tree = draw();
  return { rt, env, draw, tree, get imports() { return imports; } };
}
let guardCases = 0;
for (const dev of [false,true]) for (const flag of [undefined,'0','1','true']) {
  flags(dev, flag); const f = appFixture(), enabled = dev && flag === '1';
  assert.equal(f.tree.type, enabled ? 'CheckEmailPreviewScreen' : 'WelcomeScreen'); assert.equal(f.imports, enabled ? 1 : 0);
  const rt = runtime(), env = environment(); const { CheckEmailPreviewScreen } = loader(rt,env)('pages/CheckEmailPreviewScreen.tsx');
  assert.equal(rt.render(()=>presentation(CheckEmailPreviewScreen({onBack(){},onDifferentNic(){},onLogin(){},onExit(){}}))) === null, !enabled, 'direct mount guard');
  f.rt.unmount(); rt.unmount(); guardCases++;
}
flags(true,'1','1'); const both = appFixture(); assert.equal(both.tree.type,'AccountCreatedPreviewScreen'); assert.equal(both.imports,0); both.tree.props.onExit(); assert.equal(both.draw().type,'WelcomeScreen'); both.rt.unmount();
for (const action of ['onBack','onDifferentNic','onExit','androidBack','onLogin']) {
  flags(true,'1'); const f=appFixture();
  if(action==='androidBack') assert(f.env.back()); else f.tree.props[action]();
  let tree=f.draw(); assert.equal(tree.type,action==='onExit'?'WelcomeScreen':action==='onLogin'?'LoginScreen':'ForgotPasswordScreen');
  if(action!=='onExit'){if(action!=='onLogin'){tree.props.onLogin();tree=f.draw();}assert.equal(tree.type,'LoginScreen');tree.props.onBack();tree=f.draw();assert.equal(tree.type,'WelcomeScreen','standalone preview preserves initial Login origin');}
  f.rt.unmount();
}
function fixture(width=400,height=772,fontScale=1,overrides={},mode='standalone') {
  const rt=runtime(),env=environment(width,height,fontScale),load=loader(rt,env,overrides);
  const component=load('pages/CheckEmailPreviewScreen.tsx')[mode==='demo'?'CheckEmailDemoScreen':'CheckEmailPreviewScreen'];
  let tree,backs=0,others=0,logins=0,exits=0;
  const draw=()=>{tree=rt.render(()=>presentation(component({onBack(){backs++;},onDifferentNic(){others++;},onLogin(){logins++;},onExit(){exits++;}})));rt.effects();};draw();
  function measure(cardHeight=440*fontScale,visionHeight=47*fontScale){
    nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).flex===1).props.onLayout({nativeEvent:{layout:{width:Math.min(480,width-32),height:height-80}}});
    nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).width===86).props.onLayout({nativeEvent:{layout:{height:visionHeight}}});
    nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).borderRadius===24).props.onLayout({nativeEvent:{layout:{height:cardHeight}}});draw();
  }
  return {rt,env,draw,measure,get tree(){return tree;},get counts(){return{backs,others,logins,exits};},unmount(){rt.unmount();}};
}
let layouts=0;
for(const [width,height]of [[320,568],[360,640],[360,800],[412,915],[600,960],[400,772]])for(const fontScale of[1,2,3]){
  flags(true,'1');const f=fixture(width,height,fontScale);f.measure();const writes=f.rt.writes;for(let i=0;i<4;i++)f.measure();assert.equal(f.rt.writes,writes,'measured layout converges');
  const tree=f.tree,texts=nodes(tree,'Text').map(n=>n.props.children);assert(texts.includes('sample@example.test'));assert(texts.includes('Demo preview \u2014 no email was sent.'));assert(texts.includes('Sample reset-link destination:'));
  assert.equal(nodes(tree,'TextInput').length,0);const disabled=nodes(tree,'Pressable').filter(n=>n.props.disabled);assert.equal(disabled.length,1);
  for(const n of disabled){assert(n.props.accessibilityState.disabled);assert(!n.props.onPress,'disabled Resend has no handler');assert(n.props.accessibilityLabel.startsWith('Resend link.'));}
  for(const n of nodes(tree,'Pressable')){const style=flat(n.props.style);assert((style.minHeight??style.height)>=48);const targetWidth=style.minWidth??(style.width==='100%'?Math.min(480,width-32)-34:style.width??48);assert(targetWidth>=48);}
  const scroll=nodes(tree,'ScrollView')[0];assert.equal(scroll.props.showsVerticalScrollIndicator,false);assert(!('scrollEnabled'in scroll.props));
  assert(!nodes(tree,'Pressable').some(n=>n.props.accessibilityLabel==='Exit preview and return to Welcome'),'no visible Exit preview control');assert(!texts.includes('Exit preview'));assert(!nodes(tree,'Text').some(n=>n.props.onPress),'no text actions');
  const logo=nodes(tree,'View').find(n=>n.props.accessibilityRole==='image'),size=flat(logo.props.style);assert(Math.abs(size.height/size.width-151.531/150)<1e-5);assert(size.width/2+4<=Math.min(480,width-32)/2-86-9+0.01);assert.equal(size.width,Math.min(150,width*0.375,Math.max(0,Math.min(480,width-32)-198)));
  const crop=flat(nodes(logo,'Image')[0].props.style);assert.equal(crop.width,size.width*1.5417);assert.equal(crop.height,size.height*2.7132);assert.equal(crop.left,-size.width*0.3013);assert.equal(crop.top,-size.height*0.636);
  const back=flat(nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel==='Back to Forgot Password').props.style);assert(back.left>=0&&back.top>=0&&back.zIndex>=2);
  const card=flat(nodes(tree,'View').find(n=>n.props.onLayout&&flat(n.props.style).borderRadius===24).props.style);assert(!('height'in card)&&!('minHeight'in card)&&!('maxHeight'in card),'card stays intrinsic');
  const login=nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel==='Back to log in');assert(!login.props.disabled);assert.equal(flat(login.props.style).backgroundColor,'#8b1730');assert(texts.includes('Unavailable in demo'));assert(texts.includes('Resend link'));assert(texts.includes('Check your spam folder or try again.'));assert(!texts.includes('Open email app'));
  const badge=nodes(tree,'View').find(n=>flat(n.props.style).backgroundColor==='#edf6f1');assert.equal(flat(badge.props.style).width,64);assert.equal(flat(nodes(badge,'Image')[0].props.style).width,32);assert.equal(flat(nodes(badge,'Image')[0].props.style).tintColor,'#007a53');
  nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel==='Back to Forgot Password').props.onPress();
  nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel.startsWith('Use a different NIC.')).props.onPress();login.props.onPress();
  assert.deepEqual(f.counts,{backs:1,others:1,logins:1,exits:0});f.unmount();layouts++;
}
for(const [args,status]of [[[],0],[['--port','8082'],7],[[],null]]){
  let launch,exit;const parent={EXPO_PUBLIC_CHECK_EMAIL_PREVIEW:'0',EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW:'1',KEEP:'same'};
  vm.runInNewContext(fs.readFileSync(path.join(root,'scripts/preview-check-email.cjs'),'utf8'),{__dirname:path.join(root,'scripts'),require:n=>n==='node:path'?path:n==='node:child_process'?{spawnSync:(exe,a,options)=>(launch={exe,args:a,options},{status})}:require(n),process:{argv:['node','preview',...args],execPath:process.execPath,env:parent,exit:s=>{exit=s;}},console:{log(){},error(){}}});
  assert.deepEqual(Array.from(launch.args).slice(1),['start','--go','--lan',...args]);assert.equal(launch.options.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW,'1');assert.equal(launch.options.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW,'0');assert.equal(parent.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW,'0');assert.equal(parent.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW,'1');assert.equal(launch.options.env.KEEP,'same');assert.equal(exit,status??1);
}
const svg=fs.readFileSync(path.join(root,'assets/figma/check-email/mail.svg'));assert.equal(crypto.createHash('sha256').update(svg).digest('hex'),'cdbae6793b123dbb6e5e5f7bc111d84df0fd7eaa03563a07d5d030ae96620bcb');const png=fs.readFileSync(path.join(root,'assets/figma/check-email/mail.png'));assert.equal(png.subarray(0,8).toString('hex'),'89504e470d0a1a0a');assert.equal(png.readUInt32BE(16),136);assert.equal(png.readUInt32BE(20),136);
if(process.argv.includes('--export')){
  let files=[];function walk(d){for(const e of fs.readdirSync(d,{withFileTypes:true})){const f=path.join(d,e.name);if(e.isDirectory())walk(f);else files.push(f);}}walk(path.resolve(root,process.argv[process.argv.indexOf('--export')+1]||'dist/android'));
  const iconHash=crypto.createHash('sha256').update(png).digest('hex');assert(!files.some(f=>crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex')===iconHash),'exclusive mail asset excluded from production');
  const bundles=files.filter(f=>/\.(hbc|js)$/.test(f));assert(bundles.length);for(const f of bundles){const b=fs.readFileSync(f);for(const marker of ['sample@example.test','CheckEmailPreviewScreen','No email sent','Demo preview \u2014 no email was sent.'])assert(!b.includes(Buffer.from(marker))&&!b.includes(Buffer.from(marker,'utf16le')),'preview marker in release');}
  console.log('PASS current production artifact: Check Email module/sample copy/mail asset excluded.');
}
flags(previous.dev,previous.email,previous.account);
console.log(`PASS Check Email: ${guardCases} development/flag cases, Account Created precedence, direct guard, all exit routes, static sample/disabled actions/no side effects, launcher port/env/exit handling, ${layouts} synthetic layout/convergence cases. No native/device rendering.`);

module.exports = { fixture };
