// Mocked React/native behavior and artifact inspection; no device rendering claim.
const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict'), vm = require('node:vm'), crypto = require('node:crypto');
const root = path.resolve(__dirname, '..');
global.__DEV__ = false;
const { runtime, environment, loader, nodes } = require('./check-register.cjs');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex');
const flat = style => Object.assign({}, ...[style].flat(Infinity).filter(Boolean));
const source = read('PasswordUpdatedPreviewScreen.tsx');
assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|Linking|console|setTimeout|setInterval)\b/.test(source));
assert(!/allowFontScaling=\{false\}|maxFontSizeMultiplier|numberOfLines/.test(source));
for (const file of ['ResetPasswordPreviewScreen.tsx','ForgotPasswordScreen.tsx','CheckEmailPreviewScreen.tsx']) assert(!/PASSWORD_UPDATED_PREVIEW|PasswordUpdatedPreview/.test(read(file)), 'no false success entry: ' + file);
const keys = ['EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW','EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW','EXPO_PUBLIC_CHECK_EMAIL_PREVIEW','EXPO_PUBLIC_RESET_PASSWORD_PREVIEW'];
const saved = keys.map(k => process.env[k]), savedDev = global.__DEV__;
function flags(dev, ...values) { global.__DEV__ = dev; keys.forEach((k,i) => values[i] === undefined ? delete process.env[k] : process.env[k] = values[i]); }
function appFixture() {
  const rt = runtime(), env = environment(), overrides = {}; let imports = 0;
  for (const [file, asset] of [['WelcomeScreen','welcomeAssets'],['LoginScreen','loginAssets'],['RegisterScreen','registerAssets'],['ForgotPasswordScreen','forgotPasswordAssets'],['OnboardingMeetScreen','onboardingAssets'],['OnboardingDiscoverScreen','discoverAssets'],['OnboardingAskScreen','askAssets'],['OnboardingTrackScreen','trackAssets'],['OnboardingPayScreen','payAssets']]) overrides['./'+file] = { [file]: file, [asset]: { hero: 1 } };
  for (const file of ['AccountCreatedPreviewScreen','CheckEmailPreviewScreen','ResetPasswordPreviewScreen']) overrides['./'+file] = { [file]: file };
  overrides['./PasswordUpdatedPreviewScreen'] = { get PasswordUpdatedPreviewScreen() { imports++; return 'PasswordUpdatedPreviewScreen'; } };
  const { Startup } = loader(rt, env, overrides)('App.tsx');
  const draw = () => { const t = rt.render(() => Startup()); rt.effects(); return t; };
  let tree = draw(); assert.notEqual(tree.type, 'PasswordUpdatedPreviewScreen', 'resource readiness gate');
  nodes(tree,'Image').forEach(n => n.props.onLoad?.()); tree = draw();
  return { rt, env, draw, tree, get imports() { return imports; } };
}
let guards = 0;
for (const dev of [false,true]) for (const own of [undefined,'0','1','true']) for (const account of ['0','1']) for (const email of ['0','1']) for (const reset of ['0','1']) {
  flags(dev, own, account, email, reset);
  const f = appFixture(), enabled = dev && own === '1' && account !== '1' && email !== '1' && reset !== '1';
  const expected = !dev ? 'WelcomeScreen' : account === '1' ? 'AccountCreatedPreviewScreen' : email === '1' ? 'CheckEmailPreviewScreen' : reset === '1' ? 'ResetPasswordPreviewScreen' : enabled ? 'PasswordUpdatedPreviewScreen' : 'WelcomeScreen';
  assert.equal(f.tree.type, expected); assert.equal(f.imports, enabled ? 1 : 0);
  if (enabled) {
    f.tree.props.onLogin(); let t = f.draw(); assert.equal(t.type, 'LoginScreen');
    t.props.onBack(); t = f.draw(); assert.equal(t.type, 'WelcomeScreen');
  } else if (dev && account === '1') { f.tree.props.onExit(); assert.equal(f.draw().type, 'WelcomeScreen'); }
  else if (dev && email === '1') { f.tree.props.onBack(); assert.equal(f.draw().type, 'ForgotPasswordScreen'); }
  else if (dev && reset === '1') { f.tree.props.onExit(); assert.equal(f.draw().type, 'WelcomeScreen'); }
  const rt = runtime(), env = environment(), { PasswordUpdatedPreviewScreen } = loader(rt, env)('PasswordUpdatedPreviewScreen.tsx');
  assert.equal(PasswordUpdatedPreviewScreen({ onLogin() {} }) === null, !(dev && own === '1'), 'direct guard');
  f.rt.unmount(); guards++;
}
flags(true,'1','0','0','0');
{
  const f = appFixture(); assert(f.env.back()); assert.equal(f.draw().type, 'WelcomeScreen'); f.rt.unmount();
}
let layouts = 0;
for (const [width,height] of [[320,568],[360,640],[360,800],[412,915],[600,960],[400,772]]) for (const fontScale of [1,2,3]) {
  const rt = runtime(), env = environment(width,height,fontScale);
  const { PasswordUpdatedPreviewScreen } = loader(rt, env)('PasswordUpdatedPreviewScreen.tsx');
  let logins = 0; const onLogin = () => logins++, wrapper = PasswordUpdatedPreviewScreen({ onLogin });
  let tree; const draw = () => { tree = rt.render(() => wrapper.type(wrapper.props)); rt.effects(); }; draw();
  function measure() {
    nodes(tree,'View').find(n => n.props.onLayout && flat(n.props.style).flex === 1).props.onLayout({ nativeEvent:{ layout:{ width:Math.min(480,width-32), height:height-80 } } });
    nodes(tree,'Text').find(n => n.props.onLayout).props.onLayout({ nativeEvent:{ layout:{ height:18*fontScale } } });
    nodes(tree,'View').find(n => n.props.onLayout && flat(n.props.style).width === 86).props.onLayout({ nativeEvent:{ layout:{ height:47*fontScale } } });
    nodes(tree,'View').find(n => n.props.onLayout && flat(n.props.style).paddingVertical === 24).props.onLayout({ nativeEvent:{ layout:{ height:271*fontScale } } }); draw();
  }
  measure(); const writes = rt.writes; for(let i=0;i<3;i++) measure(); assert.equal(rt.writes,writes, 'measurement convergence');
  const texts = nodes(tree,'Text').map(n => n.props.children);
  assert(texts.includes('Preview · No password was changed'));
  assert(texts.includes('Password updated'));
  assert(texts.includes('This is a design preview. No password was changed and no devices were signed out.'));
  assert(!texts.includes('Exit preview')); assert.equal(nodes(tree,'TextInput').length,0);
  const actions = nodes(tree,'Pressable'); assert.equal(actions.length,1); assert.equal(actions[0].props.accessibilityRole,'button');
  const action = flat(actions[0].props.style); assert(action.minHeight>=48); assert.equal(action.width,'100%'); assert.equal(action.backgroundColor,'#8b1a2e');
  actions[0].props.onPress(); assert.equal(logins,1);
  const scroll = nodes(tree,'ScrollView')[0]; assert.equal(scroll.props.showsVerticalScrollIndicator,false); assert.equal(scroll.props.showsHorizontalScrollIndicator,false); assert(!('scrollEnabled' in scroll.props));
  const badge = nodes(tree,'View').find(n => flat(n.props.style).backgroundColor === '#f2f8f4'); assert.equal(flat(badge.props.style).width,72); assert.equal(flat(nodes(badge,'Image')[0].props.style).width,34);
  const logo = nodes(tree,'View').find(n => n.props.accessibilityRole === 'image'), ls = flat(logo.props.style), img = flat(nodes(logo,'Image')[0].props.style);
  assert(Math.abs(ls.height/ls.width-151.531/150)<1e-5); assert(Math.abs(img.width/ls.width-1.5417)<1e-5); assert(Math.abs(img.height/ls.height-2.7132)<1e-5);
  const card = flat(nodes(tree,'View').find(n => n.props.onLayout && flat(n.props.style).paddingVertical === 24).props.style); assert(!('height' in card));
  rt.unmount(); layouts++;
}
for(const args of [[],['--port','8084']]) {
  let launch, exit; const parent = Object.fromEntries(keys.map(k => [k,'1'])); parent.EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW = '0';
  vm.runInNewContext(read('scripts/preview-password-updated.cjs'), { __dirname:path.join(root,'scripts'), require:n=>n==='node:path'?path:{spawnSync:(exe,args,options)=>(launch={args,options},{status:7})}, process:{argv:['node','preview',...args],execPath:process.execPath,env:parent,exit:s=>{exit=s;}},console:{log(){},error(){}} });
  assert.deepEqual(Array.from(launch.args).slice(1),['start','--go','--lan',...args]); assert.equal(exit,7); assert.equal(parent.EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW,'0');
  assert.equal(launch.options.env.EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW,'1'); for(const k of keys.slice(1)) assert.equal(launch.options.env[k],'0');
}
for(const args of [['--dev'],['--port','0'],['--port','65536'],['--port','8084','extra']]) assert.throws(()=>vm.runInNewContext(read('scripts/preview-password-updated.cjs'),{process:{argv:['node','preview',...args]},require:n=>n==='node:path'?path:{},console}));
const iconFile='assets/figma/password-updated/shield-check.png';
assert.equal(hash(iconFile),'02bb59e835016e7a6b5f1c1e0c02b5bee43343713f6de0552b702eb1157684f5');
const png=fs.readFileSync(path.join(root,iconFile)); assert.equal(png.readUInt32BE(16),136); assert.equal(png.readUInt32BE(20),136);
if(process.argv.includes('--export')) {
  const dir=path.resolve(root,process.argv[process.argv.indexOf('--export')+1]||'dist/android'), files=[];
  function walk(d){for(const e of fs.readdirSync(d,{withFileTypes:true})){const p=path.join(d,e.name);e.isDirectory()?walk(p):files.push(p);}} walk(dir);
  assert(!files.some(f=>crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex')===hash(iconFile)),'exclusive shield excluded');
  const bundles=files.filter(f=>/\.(hbc|js)$/.test(f)); assert(bundles.length);
  for(const f of bundles)for(const marker of ['PasswordUpdatedPreviewScreen','Preview · No password was changed','This is a design preview. No password was changed and no devices were signed out.']){const b=fs.readFileSync(f);assert(!b.includes(Buffer.from(marker))&&!b.includes(Buffer.from(marker,'utf16le')),'preview copy/module excluded');}
  console.log('PASS production module/copy/shield exclusion:',dir);
}
const snapshot=path.join(root,'.cache/password-updated-before.json');
if(fs.existsSync(snapshot))for(const [f,h]of Object.entries(JSON.parse(fs.readFileSync(snapshot,'utf8'))))if(!['App.tsx','package.json'].includes(f))assert.equal(hash(f),h,'existing source unchanged: '+f);
flags(savedDev,...saved);
console.log(`PASS Password Updated: ${guards} guards/precedence cases, fresh Login/Back routes, no false success entry, one control, ${layouts} synthetic layout/convergence cases, launcher isolation/arguments, exact shield hash. No native/device rendering.`);
