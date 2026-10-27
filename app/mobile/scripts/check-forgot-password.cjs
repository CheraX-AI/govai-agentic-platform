// Uses synthetic React/native measurements. It is not native layout/device verification.
const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict'), crypto = require('node:crypto');
global.__DEV__ = false;
const { runtime, environment, loader, nodes } = require('./check-register.cjs');
const root = path.resolve(__dirname, '..'), flat = style => Object.assign({}, ...[style].flat(Infinity).filter(Boolean));
const source = fs.readFileSync(path.join(root, 'ForgotPasswordScreen.tsx'), 'utf8');
assert(!/\b(fetch|XMLHttpRequest|axios|AsyncStorage|SecureStore|localStorage|console)\b/.test(source), 'no requests/storage/logging');
assert(!/allowFontScaling=\{false\}|maxFontSizeMultiplier|numberOfLines|autoFocus|maxLength/.test(source));
assert(!/reset token|OTP|We.ll email|Divisional Secretariat/.test(source));
const notice = 'Password recovery is not connected. No request was sent and no password was changed.';
function fixture(width = 400, height = 772, fontScale = 1) {
  const rt = runtime(), env = environment(width, height, fontScale);
  const component = loader(rt, env)('ForgotPasswordScreen.tsx');
  let tree, exits = 0, clears = 0, focuses = 0, viewportHeight = height - 80, fieldY = 300, fieldHeight = 72 * fontScale;
  const scrolls = [], controls = {};
  function draw() {
    tree = rt.render(() => component.ForgotPasswordScreen({ onLogin() { exits++; } }));
    for (const n of nodes(tree)) if (n.props.ref) {
      let control;
      if (n.type === 'TextInput') control = controls.input ?? (controls.input = { clear() { clears++; }, focus() { focuses++; nodes(tree, 'TextInput')[0].props.onFocus(); } });
      else if (n.type === 'ScrollView') control = { scrollTo(point) { scrolls.push(point); } };
      else if (nodes(n, 'ScrollView').length) control = { measureInWindow(fn) { fn(16, 46, Math.min(480, width - 32), viewportHeight); } };
      else control = { measureInWindow(fn) { fn(16, fieldY, Math.min(480, width - 32), fieldHeight); } };
      n.props.ref.current = control;
    }
    rt.effects(); return tree;
  }
  const text = () => nodes(tree, 'Text').flatMap(n => [n.props.children].flat(Infinity)).filter(s => typeof s === 'string');
  const header = () => flat(nodes(tree, 'View').find(n => flat(n.props.style).flexShrink === 0 && flat(n.props.style).alignItems === 'center').props.style).height;
  function layout(h = height - 80, card = 430 * fontScale, w = Math.min(480, width - 32)) {
    viewportHeight = h;
    nodes(tree, 'View').find(n => n.props.ref && nodes(n, 'ScrollView').length).props.onLayout({ nativeEvent: { layout: { width: w, height: h } } });
    nodes(tree, 'View').find(n => flat(n.props.style).paddingVertical === 24).props.onLayout({ nativeEvent: { layout: { height: card } } });
    nodes(tree, 'View').find(n => flat(n.props.style).width === 86).props.onLayout({ nativeEvent: { layout: { height: 47 * fontScale } } });
    rt.flush(); draw();
  }
  function set(value) { nodes(tree, 'TextInput')[0].props.onChangeText(value); draw(); }
  function submit() { nodes(tree, 'Pressable').find(n => n.props.accessibilityLabel.startsWith('Send reset link.')).props.onPress(); draw(); rt.flush(); draw(); }
  function show(top) { env.visible = true; env.metrics = { screenY: top }; env.listeners.keyboardDidShow({ endCoordinates: env.metrics }); rt.flush(); draw(); }
  function hide() { env.visible = false; env.metrics = undefined; env.listeners.keyboardDidHide(); rt.flush(); draw(); }
  function leave(label = 'Back to Login') { nodes(tree, 'Pressable').find(n => n.props.accessibilityLabel === label).props.onPress(); draw(); }
  function unmount() { rt.unmount(); assert.equal(rt.frames.size, 0); assert.equal(rt.timers.size, 0); }
  draw();
  return { rt, env, draw, layout, set, submit, show, hide, leave, unmount, text, header, scrolls, field(y, h) { fieldY = y; fieldHeight = h; },
    get tree() { return tree; }, get clears() { return clears; }, get focuses() { return focuses; }, get exits() { return exits; } };
}
for (const exit of ['Back to Login', 'Log in']) {
  const f = fixture(); assert.equal(nodes(f.tree, 'TextInput')[0].props.value, '');
  f.submit(); assert.equal(f.focuses, 1); assert(f.text().includes('Enter your National ID number.'));
  f.set('  '); f.submit(); assert(!f.text().includes(notice));
  f.set(' arbitrary NIC / no format rule '); f.submit(); assert(f.text().includes(notice));
  assert.equal(nodes(f.tree, 'TextInput')[0].props.value, ' arbitrary NIC / no format rule ');
  assert(!f.text().includes('Enter your National ID number.'));
  f.leave(exit); assert.equal(f.exits, 1); assert.equal(nodes(f.tree, 'TextInput')[0].props.value, ''); assert(!f.text().includes(notice));
  assert(!f.text().includes('Enter your National ID number.')); f.unmount(); assert(f.clears >= 2);
}
let layouts = 0, keyboards = 0;
for (const [width, height] of [[320,568],[360,640],[360,800],[412,915],[600,960],[400,772]]) for (const scale of [1,2,3]) {
  const f = fixture(width, height, scale); f.layout();
  const writes = f.rt.writes; for (let i = 0; i < 4; i++) f.layout(); assert.equal(f.rt.writes, writes, 'intrinsic measurements converge');
  const scroll = nodes(f.tree, 'ScrollView')[0]; assert.equal(scroll.props.showsVerticalScrollIndicator, false); assert.equal(scroll.props.automaticallyAdjustKeyboardInsets, false);
  assert(!('scrollEnabled' in scroll.props), 'overflow remains natural');
  for (const button of nodes(f.tree, 'Pressable')) { const s = flat(button.props.style); assert((s.minHeight ?? s.height) >= 48); assert((s.minWidth ?? s.width ?? 48) >= 48); }
  const logo = nodes(f.tree, 'View').find(n => n.props.accessibilityRole === 'image'), size = flat(logo.props.style);
  assert(Math.abs(size.height / size.width - 151.531 / 150) < 1e-5); assert(size.width <= 150);
  assert(size.width / 2 + 4 <= Math.min(480, width - 32) / 2 - 86 - 9 + 0.01, 'logo reserves the vision offset and separation');
  const stable = f.header(); nodes(f.tree, 'TextInput')[0].props.onFocus(); f.layout((height - 80) / 2); assert.equal(f.header(), stable, 'focus guard accepts early Android resize without shrinking header');
  f.show(300); f.field(350, 72 * scale); f.layout((height - 80) / 2); assert.equal(f.header(), stable, 'keyboard resize preserves decoration baseline');
  assert(f.scrolls.length > 0, 'focused field/error can be revealed');
  const padding = flat(nodes(f.tree, 'ScrollView')[0].props.contentContainerStyle).paddingBottom;
  assert.equal(padding, Math.max(0, 46 + (height - 80) / 2 - 300) + 8, 'only actual overlay overlap is compensated');
  f.hide(); f.layout(); assert.equal(f.header(), stable, 'keyboard closing restores same baseline'); keyboards++;
  f.show(300); f.layout((height - 80) / 2); f.hide(); f.layout((height - 80) / 2);
  const timers = [...f.rt.timers.entries()]; timers.forEach(([id, fn]) => { f.rt.timers.delete(id); fn(); }); f.rt.flush(); f.draw();
  assert(f.header() <= stable, 'settled genuine height decrease is accepted'); keyboards++;
  f.unmount(); layouts++;
}
// Execute actual Login link handler: clear sensitive values and visibility before navigation.
{
  const rt = runtime(), env = environment(), { LoginScreen } = loader(rt, env)('LoginScreen.tsx'); let tree, opened = 0, cleared = 0;
  function draw() { tree = rt.render(() => LoginScreen({ onBack() {}, onSignUp() {}, onForgotPassword() { opened++; } }));
    for (const n of nodes(tree)) if (n.props.ref && typeof n.props.ref !== 'function') n.props.ref.current = { clear() { cleared++; }, focus() {}, measureInWindow(fn) { fn(16,46,328,600); }, scrollTo() {} };
    rt.effects(); }
  draw(); nodes(tree,'TextInput')[0].props.onChangeText('fake UI identifier'); nodes(tree,'TextInput')[1].props.onChangeText('disposable UI password'); draw();
  nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel==='Show password').props.onPress(); draw();
  const link = nodes(tree,'Pressable').find(n=>n.props.accessibilityLabel==='Forgot Password?'); assert(!link.props.disabled); link.props.onPress(); draw();
  assert.equal(opened,1); assert(nodes(tree,'TextInput').every(n=>n.props.value==='')); assert(nodes(tree,'TextInput')[1].props.secureTextEntry); assert(cleared>=2); rt.unmount();
}
const svgHashes = { lock:'c902bc35d45c6353bc47b1ac32badc4f168ce83ba47295d846b6d58554600224', identity:'9f7febe0566315dcece1521663a7360e1995e01c403a142fb69007de2d76c9ee', info:'72bd977ef6c31bf0d67b32edf688854770b9d0043ad4c0e95e17927afea4a982' };
for (const [name, hash] of Object.entries(svgHashes)) { const file=path.join(root,'assets/figma/forgot-password',name); assert.equal(crypto.createHash('sha256').update(fs.readFileSync(file+'.svg')).digest('hex'),hash); const png=fs.readFileSync(file+'.png'); assert.equal(png.subarray(0,8).toString('hex'),'89504e470d0a1a0a'); }
console.log(`PASS Forgot Password: empty/required-only/raw NIC, approved disconnected copy, no requests or simulated success, explicit/unmount cleanup, Login cleanup and six-origin navigation; ${layouts} synthetic layout/convergence and ${keyboards} keyboard cases; original asset hashes. Native/device rendering remains untested.`);
