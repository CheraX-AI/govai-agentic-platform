// Component logic and artifact checks; mocks do not prove native rendering.
const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict'), vm = require('node:vm'), crypto = require('node:crypto');
const root = path.resolve(__dirname, '..');
global.__DEV__ = false;
const { runtime, environment, loader, nodes } = require('./check-register.cjs');
const flat = style => Object.assign({}, ...[style].flat(Infinity).filter(Boolean));
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const source = read('ResetPasswordPreviewScreen.tsx');
assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|Linking|console)\b/.test(source));
assert(!/\.trim\(|maxLength|allowFontScaling=\{false\}|maxFontSizeMultiplier|numberOfLines/.test(source));
assert(!/RESET_PASSWORD_PREVIEW|ResetPasswordPreview/.test(read('ForgotPasswordScreen.tsx')));
assert(!/RESET_PASSWORD_PREVIEW|ResetPasswordPreview/.test(read('CheckEmailPreviewScreen.tsx')));
const flagKeys = ['EXPO_PUBLIC_RESET_PASSWORD_PREVIEW', 'EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW', 'EXPO_PUBLIC_CHECK_EMAIL_PREVIEW'];
const previous = flagKeys.map(k => process.env[k]), previousDev = global.__DEV__;
function flags(dev, reset, account = '0', email = '0') {
  global.__DEV__ = dev;
  [reset, account, email].forEach((v, i) => v === undefined ? delete process.env[flagKeys[i]] : process.env[flagKeys[i]] = v);
}
function appFixture() {
  const rt = runtime(), env = environment(), overrides = {}; let imports = 0;
  for (const [file, asset] of [['WelcomeScreen','welcomeAssets'],['LoginScreen','loginAssets'],['RegisterScreen','registerAssets'],['ForgotPasswordScreen','forgotPasswordAssets'],['OnboardingMeetScreen','onboardingAssets'],['OnboardingDiscoverScreen','discoverAssets'],['OnboardingAskScreen','askAssets'],['OnboardingTrackScreen','trackAssets'],['OnboardingPayScreen','payAssets']]) overrides['./' + file] = { [file]: file, [asset]: { hero: 1 } };
  overrides['./AccountCreatedPreviewScreen'] = { AccountCreatedPreviewScreen: 'AccountCreatedPreviewScreen' };
  overrides['./CheckEmailPreviewScreen'] = { CheckEmailPreviewScreen: 'CheckEmailPreviewScreen' };
  overrides['./ResetPasswordPreviewScreen'] = { get ResetPasswordPreviewScreen() { imports++; return 'ResetPasswordPreviewScreen'; } };
  const { Startup } = loader(rt, env, overrides)('App.tsx');
  const draw = () => { const tree = rt.render(() => Startup()); rt.effects(); return tree; };
  let tree = draw(); assert.notEqual(tree.type, 'ResetPasswordPreviewScreen');
  nodes(tree, 'Image').forEach(n => n.props.onLoad?.()); tree = draw();
  return { rt, env, draw, tree, get imports() { return imports; } };
}
let guards = 0;
for (const dev of [false, true]) for (const reset of [undefined, '0', '1', 'true']) for (const account of ['0', '1']) for (const email of ['0', '1']) {
  flags(dev, reset, account, email);
  const f = appFixture(), enabled = dev && reset === '1' && account !== '1' && email !== '1';
  assert.equal(f.tree.type, dev && account === '1' ? 'AccountCreatedPreviewScreen' : dev && email === '1' ? 'CheckEmailPreviewScreen' : enabled ? 'ResetPasswordPreviewScreen' : 'WelcomeScreen');
  assert.equal(f.imports, enabled ? 1 : 0);
  if (enabled) { f.tree.props.onExit(); assert.equal(f.draw().type, 'WelcomeScreen'); }
  const rt = runtime(), env = environment(), { ResetPasswordPreviewScreen } = loader(rt, env)('ResetPasswordPreviewScreen.tsx');
  assert.equal(ResetPasswordPreviewScreen({ onExit() {} }) === null, !(dev && reset === '1'));
  f.rt.unmount(); rt.unmount(); guards++;
}
flags(true, '1');
function fixture(width = 400, height = 772, fontScale = 1) {
  const rt = runtime(), env = environment(width, height, fontScale);
  const { ResetPasswordPreviewScreen } = loader(rt, env)('ResetPasswordPreviewScreen.tsx');
  let exits = 0, tree, viewportHeight = height - 80;
  const onExit = () => { exits++; }, wrapper = ResetPasswordPreviewScreen({ onExit });
  const controls = [], clears = [], focuses = [], scrolls = [];
  function draw() {
    tree = rt.render(() => wrapper.type(wrapper.props));
    const fields = nodes(tree, 'TextInput');
    for (const n of nodes(tree)) if (n.props.ref) {
      let control;
      if (n.type === 'TextInput') {
        const index = fields.indexOf(n);
        control = controls[index] ??= { clear() { clears.push(index); }, focus() { focuses.push(index); nodes(tree, 'TextInput')[index].props.onFocus(); } };
      } else if (n.type === 'ScrollView') control = { scrollTo: p => scrolls.push(p) };
      else {
        const child = typeof n.props.ref === 'function' ? nodes(n, 'TextInput')[0] : undefined, index = fields.indexOf(child);
        control = { measureInWindow: fn => child ? fn(16, 200 + index * 240, width - 32, 75 * fontScale) : fn(16, 46, width - 32, viewportHeight) };
      }
      if (typeof n.props.ref === 'function') n.props.ref(control); else n.props.ref.current = control;
    }
    rt.effects(); return tree;
  }
  draw();
  function set(i, value) { nodes(tree, 'TextInput')[i].props.onChangeText(value); draw(); }
  function submit() { nodes(tree, 'Pressable').find(n => n.props.accessibilityLabel.startsWith('Reset password.')).props.onPress(); draw(); rt.flush(); draw(); }
  function toggle(label) { nodes(tree, 'Pressable').find(n => n.props.accessibilityLabel === label).props.onPress(); draw(); }
  function layout(h = height - 80) {
    viewportHeight = h;
    nodes(tree, 'View').find(n => n.props.ref && typeof n.props.ref !== 'function' && n.props.onLayout).props.onLayout({ nativeEvent: { layout: { width: Math.min(480, width - 32), height: h } } });
    nodes(tree, 'View').find(n => n.props.onLayout && flat(n.props.style).paddingVertical === 24).props.onLayout({ nativeEvent: { layout: { height: 550 * fontScale } } });
    nodes(tree, 'View').find(n => n.props.onLayout && flat(n.props.style).width === 86).props.onLayout({ nativeEvent: { layout: { height: 47 * fontScale } } });
    rt.flush(); draw();
  }
  const text = () => nodes(tree, 'Text').flatMap(n => [n.props.children].flat(Infinity)).filter(x => typeof x === 'string');
  return { rt, env, draw, set, submit, toggle, layout, text, controls, clears, focuses, scrolls, get tree() { return tree; }, get exits() { return exits; } };
}
const f = fixture();
assert(!nodes(f.tree, 'View').some(n => flat(n.props.style).borderColor === '#a30f2d'), 'no red outlines before validation');
assert.equal(f.scrolls.length, 0, 'no forced scroll on entry');
assert.deepEqual(nodes(f.tree, 'TextInput').map(n => n.props.value), ['', '']);
assert(nodes(f.tree, 'TextInput').every(n => n.props.secureTextEntry));
f.submit(); assert.equal(f.focuses.at(-1), 0); assert(f.text().includes('Enter a new password.')); assert(f.text().includes('Confirm your new password.'));
assert.equal(nodes(f.tree, 'View').filter(n => flat(n.props.style).borderColor === '#a30f2d').length, 2);
f.set(0, ' a '); f.submit(); assert.equal(f.focuses.at(-1), 1);
f.set(1, 'a'); f.submit(); assert(f.text().includes('Passwords do not match.'));
f.set(1, ' a '); f.submit(); assert(f.text().includes('Password recovery is not connected. No request was sent and no password was changed.')); assert.equal(f.exits, 0);
f.set(0, ' '); f.set(1, ' '); f.submit(); assert(f.text().includes('Password recovery is not connected. No request was sent and no password was changed.'));
f.set(0, 'A'); f.set(1, 'a'); f.submit(); assert(f.text().includes('Passwords do not match.'));
f.toggle('Show new password'); assert(!nodes(f.tree, 'TextInput')[0].props.secureTextEntry); assert(nodes(f.tree, 'TextInput')[1].props.secureTextEntry);
f.toggle('Show confirmation password'); assert(nodes(f.tree, 'TextInput').every(n => !n.props.secureTextEntry));
f.toggle('Hide new password'); assert(nodes(f.tree, 'TextInput')[0].props.secureTextEntry); assert(!nodes(f.tree, 'TextInput')[1].props.secureTextEntry);
nodes(f.tree, 'TextInput')[0].props.onSubmitEditing(); assert.equal(f.focuses.at(-1), 1);
f.env.visible = true; assert(f.env.back()); assert.equal(f.exits, 0); assert(!f.env.visible);
assert(f.env.back()); f.draw(); assert.equal(f.exits, 1); assert.deepEqual(nodes(f.tree, 'TextInput').map(n => n.props.value), ['', '']);
assert(nodes(f.tree, 'TextInput').every(n => n.props.secureTextEntry)); assert(!f.text().includes('Passwords do not match.')); assert(f.clears.includes(0) && f.clears.includes(1));
f.rt.unmount(); assert.equal(f.rt.frames.size, 0); assert.equal(f.rt.timers.size, 0);
let layouts = 0;
for (const [w, h] of [[320,568],[360,640],[360,800],[412,915],[600,960],[400,772]]) for (const scale of [1,2,3]) {
  const f = fixture(w, h, scale); f.layout(); const writes = f.rt.writes; f.layout(); assert.equal(f.rt.writes, writes);
  const scroll = nodes(f.tree, 'ScrollView')[0]; assert.equal(scroll.props.showsVerticalScrollIndicator, false); assert(!('scrollEnabled' in scroll.props)); assert.equal(scroll.props.automaticallyAdjustKeyboardInsets, false);
  assert.equal(nodes(f.tree, 'Pressable').length, 3, 'two toggles and submit only');
  for (const n of nodes(f.tree, 'Pressable')) assert((flat(n.props.style).minHeight ?? flat(n.props.style).height) >= 48);
  assert(f.text().includes('Strength not evaluated')); assert(f.text().includes('Design guidance — pending backend confirmation'));
  assert(!nodes(f.tree, 'Text').some(n => n.props.children === 'Strong'));
  const logo = flat(nodes(f.tree, 'View').find(n => n.props.accessibilityRole === 'image').props.style);
  assert(Math.abs(logo.height / logo.width - 151.531 / 150) < 1e-5);
  const initialHeader = flat(nodes(f.tree, 'View').find(n => flat(n.props.style).height && flat(n.props.style).alignItems === 'center' && flat(n.props.style).width === '100%').props.style).height;
  f.env.visible = true; f.env.metrics = { screenY: 400 }; f.env.listeners.keyboardDidShow({ endCoordinates: f.env.metrics });
  nodes(f.tree, 'TextInput')[1].props.onFocus(); f.rt.flush(); f.draw();
  assert(f.scrolls.length > 0, 'occluded focused field gets a conditional visibility adjustment');
  assert.equal(flat(nodes(f.tree, 'ScrollView')[0].props.contentContainerStyle).paddingBottom, Math.max(0, 46 + h - 80 - 400) + 8, 'overlay adds only actual overlap');
  f.layout(354); // Resized viewport ends at keyboard top: zero additional overlap.
  assert.equal(flat(nodes(f.tree, 'ScrollView')[0].props.contentContainerStyle).paddingBottom, 8);
  const currentHeader = flat(nodes(f.tree, 'View').find(n => flat(n.props.style).height && flat(n.props.style).alignItems === 'center' && flat(n.props.style).width === '100%').props.style).height;
  assert.equal(currentHeader, initialHeader, 'keyboard resize preserves closed layout baseline');
  f.rt.unmount(); layouts++;
}
for (const args of [[], ['--port','8083']]) {
  let launch, status;
  const parent = { EXPO_PUBLIC_RESET_PASSWORD_PREVIEW: '0', EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW: '1', EXPO_PUBLIC_CHECK_EMAIL_PREVIEW: '1' };
  vm.runInNewContext(read('scripts/preview-reset-password.cjs'), { __dirname: path.join(root,'scripts'), require: n => n === 'node:path' ? path : { spawnSync: (exe, args, options) => (launch = { args, options }, { status: 7 }) }, process: { argv: ['node','preview',...args], execPath: process.execPath, env: parent, exit: s => { status = s; } }, console: { log(){}, error(){} } });
  assert.deepEqual(Array.from(launch.args).slice(1), ['start','--go','--lan',...args]); assert.equal(status, 7);
  assert.equal(launch.options.env.EXPO_PUBLIC_RESET_PASSWORD_PREVIEW, '1'); assert.equal(launch.options.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW, '0'); assert.equal(launch.options.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW, '0'); assert.equal(parent.EXPO_PUBLIC_RESET_PASSWORD_PREVIEW, '0');
}
if (process.argv.includes('--export')) {
  const exportRoot = path.resolve(root, process.argv[process.argv.indexOf('--export') + 1] || 'dist/android');
  const files = []; function walk(dir) { for (const e of fs.readdirSync(dir, { withFileTypes: true })) { const p = path.join(dir,e.name); e.isDirectory() ? walk(p) : files.push(p); } } walk(exportRoot);
  const bundles = files.filter(f => /\.(js|hbc)$/.test(f)); assert(bundles.length);
  for (const file of bundles) for (const marker of ['ResetPasswordPreviewScreen','Strength not evaluated','Disconnected development preview. Use sample passwords only.','Design guidance — pending backend confirmation']) {
    const data = fs.readFileSync(file); assert(!data.includes(Buffer.from(marker)) && !data.includes(Buffer.from(marker, 'utf16le')), 'preview content in production: ' + marker);
  }
  console.log('PASS production artifact inspection:', exportRoot);
}
// Optional task-time snapshot; normal checks must also work on a fresh checkout.
if (fs.existsSync(path.join(root, '.cache/reset-preview-before.json'))) {
  const baseline = JSON.parse(read('.cache/reset-preview-before.json'));
  for (const [file, hash] of Object.entries(baseline)) if (!['App.tsx','package.json'].includes(file)) assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join(root,file))).digest('hex'), hash, 'existing source preserved: ' + file);
}
flags(previousDev, ...previous);
console.log(`PASS Reset Password: ${guards} guard/precedence cases, direct guard, raw matching, independent toggles, keyboard-first Back and clearing, ${layouts} synthetic layout/keyboard cases, launcher isolation, unchanged existing screen sources. No native/device rendering.`);
