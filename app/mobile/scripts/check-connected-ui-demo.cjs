// Actual component callbacks with synthetic React/native controls; no device rendering.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const crypto = require('node:crypto');
global.__DEV__ = false;
const { runtime, environment, loader, nodes } = require('./check-register.cjs');
const { fixture: chatFixture } = require('./check-chat-home-preview.cjs');
const root = path.resolve(__dirname, '..');
const keys = ['EXPO_PUBLIC_CONNECTED_UI_DEMO', 'EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW', 'EXPO_PUBLIC_CHECK_EMAIL_PREVIEW', 'EXPO_PUBLIC_RESET_PASSWORD_PREVIEW', 'EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW'];
const saved = keys.map(k => process.env[k]);
const savedDev = global.__DEV__;
const flat = style => Object.assign({}, ...[style].flat(Infinity).filter(Boolean));
function flags(dev, flag) {
  global.__DEV__ = dev;
  keys.forEach(k => { process.env[k] = '0'; });
  if (flag === undefined) delete process.env[keys[0]]; else process.env[keys[0]] = flag;
}
function unwrap(tree) {
  while (tree && typeof tree.type === 'function') tree = tree.type(tree.props);
  return tree;
}
function mount(name, props, width = 400, height = 772, fontScale = 1) {
  const rt = runtime(), env = environment(width, height, fontScale);
  const component = loader(rt, env)(name + '.tsx')[name];
  let tree; const controls = [], clears = [], focuses = [], scrolls = [];
  function draw() {
    tree = rt.render(() => unwrap(component(props)));
    const inputs = nodes(tree, 'TextInput');
    for (const n of nodes(tree)) if (n.props.ref) {
      let control;
      if (n.type === 'TextInput') {
        const i = inputs.indexOf(n);
        control = controls[i] ??= { clear() { clears.push(i); }, focus() { focuses.push(i); nodes(tree, 'TextInput')[i].props.onFocus?.(); } };
      } else if (n.type === 'ScrollView') control = { scrollTo(p) { scrolls.push(p); } };
      else control = { measureInWindow(fn) { fn(16, nodes(n, 'TextInput').length && !nodes(n, 'ScrollView').length ? 450 : 46, width - 32, nodes(n, 'ScrollView').length ? height - 80 : 80 * fontScale); } };
      if (typeof n.props.ref === 'function') n.props.ref(control); else n.props.ref.current = control;
    }
    rt.effects(); return tree;
  }
  draw();
  return {
    rt, env, draw, clears, focuses, scrolls,
    get tree() { return tree; },
    set(i, value) { nodes(tree, 'TextInput')[i].props.onChangeText(value); draw(); },
    submit() { const action = nodes(tree, 'Pressable').find(n => /^(Sign up\.|Send reset link\.|Log In\.)/.test(n.props.accessibilityLabel)); const handler = action.props.onPress; handler(); draw(); rt.flush(); draw(); return handler; },
    text() { return nodes(tree, 'Text').flatMap(n => [n.props.children].flat(Infinity)).filter(s => typeof s === 'string'); },
    unmount() { rt.unmount(); assert.equal(rt.frames.size, 0); assert.equal(rt.timers.size, 0); },
  };
}
function validRegister(f) { ['Sample Name', 'arbitrary NIC', 'not-an-email', 'not-a-phone', ' raw ', ' raw '].forEach((v,i) => f.set(i,v)); }
function app() {
  const rt = runtime(), env = environment(), overrides = {};
  for (const [name, asset] of [['WelcomeScreen','welcomeAssets'],['LoginScreen','loginAssets'],['RegisterScreen','registerAssets'],['ForgotPasswordScreen','forgotPasswordAssets'],['OnboardingMeetScreen','onboardingAssets'],['OnboardingDiscoverScreen','discoverAssets'],['OnboardingAskScreen','askAssets'],['OnboardingTrackScreen','trackAssets'],['OnboardingPayScreen','payAssets']]) overrides['./'+name] = { [name]:name, [asset]:{hero:1} };
  overrides['./AccountCreatedPreviewScreen'] = { AccountCreatedPreviewScreen:'AccountCreatedPreviewScreen', AccountCreatedDemoScreen:'AccountCreatedDemoScreen' };
  overrides['./CheckEmailPreviewScreen'] = { CheckEmailPreviewScreen:'CheckEmailPreviewScreen', CheckEmailDemoScreen:'CheckEmailDemoScreen' };
  overrides['./ResetPasswordPreviewScreen'] = { ResetPasswordPreviewScreen:'ResetPasswordPreviewScreen' };
  overrides['./ChatHomePreviewScreen'] = { ChatHomePreviewScreen:'ChatHomePreviewScreen', ChatHomeDemoScreen:'ChatHomeDemoScreen' };
  overrides['./PasswordUpdatedPreviewScreen'] = { PasswordUpdatedPreviewScreen:'PasswordUpdatedPreviewScreen' };
  const { Startup } = loader(rt,env,overrides)('App.tsx');
  const draw = () => { const t=rt.render(()=>Startup()); rt.effects(); return t; };
  let t=draw(); nodes(t,'Image').forEach(n=>n.props.onLoad?.());
  return {rt,env,draw,get tree(){return draw();}};
}
try {
  // Disabled modes retain disconnected submissions even if a callback is provided.
  for (const dev of [false,true]) for (const flag of [undefined,'0','1','true']) {
    flags(dev,flag); const enabled=dev && flag==='1'; const a=app(); let t=a.tree;
    t.props.onLogin();t=a.tree;t.props.onSignUp();t=a.tree;
    assert.equal(typeof t.props.onDemoValidated === 'function',enabled);
    for(const name of ['RegisterScreen','ForgotPasswordScreen','LoginScreen']) {
      let calls=0;const f=mount(name,{onLogin(){},onDemoValidated(){calls++;}});
      if(name==='RegisterScreen')validRegister(f);else if(name==='LoginScreen'){f.set(0,'sample id');f.set(1,'sample password');}else f.set(0,' arbitrary NIC ');
      f.submit();assert.equal(calls,enabled?1:0);
      if(!enabled)assert(f.text().some(s=>/No (registration |sign-in )?request was sent/.test(s)));
      f.unmount();
    }
    for(const [file,name,props] of [['AccountCreatedPreviewScreen','AccountCreatedDemoScreen',{onContinue(){}}],['CheckEmailPreviewScreen','CheckEmailDemoScreen',{onBack(){},onDifferentNic(){}}]]) {
      const rt=runtime(),env=environment(),component=loader(rt,env)(file+'.tsx')[name];
      assert.equal(component(props)===null,!enabled);
      rt.unmount();
    }
    a.rt.unmount();
  }
  flags(true,'1');
  for(const label of ['Back to previous screen','Sign up','Forgot Password?']) {
    let exits=0,calls=0;
    const f=mount('LoginScreen',{onBack(){exits++;},onSignUp(){exits++;},onForgotPassword(){exits++;},onDemoValidated(){calls++;}});
    f.set(0,'sample id');f.set(1,'sample password');
    const replay=nodes(f.tree,'Pressable').find(n=>n.props.accessibilityLabel.startsWith('Log In.')).props.onPress;
    nodes(f.tree,'Pressable').find(n=>n.props.accessibilityLabel===label).props.onPress();f.draw();
    assert.equal(exits,1);assert(nodes(f.tree,'TextInput').every(n=>n.props.value===''));
    replay();assert.equal(calls,0,'a submission from a departed form is ignored');f.unmount();
  }
  const origins=['WelcomeScreen','OnboardingMeetScreen','OnboardingDiscoverScreen','OnboardingAskScreen','OnboardingTrackScreen','OnboardingPayScreen'];
  for(let i=0;i<origins.length;i++) {
    const a=app();let t=a.tree;
    if(i){t.props.onGetStarted();t=a.tree;for(let j=1;j<i;j++){t.props.onNext();t=a.tree;}}
    assert.equal(t.type,origins[i]);t.props.onLogin();t=a.tree;
    // Keyboard first also applies on Login.
    a.env.visible=true;assert(a.env.back());t=a.tree;assert.equal(t.type,'LoginScreen');assert(!a.env.visible);
    t.props.onSignUp();t=a.tree;const registerProps=t.props;
    const f=mount('RegisterScreen',registerProps);f.submit();assert.equal(a.tree.type,'RegisterScreen');
    validRegister(f);f.set(5,'different');f.submit();assert(f.text().includes('Passwords do not match.'));
    f.set(5,' raw ');const repeat=f.submit();repeat();
    assert.equal(a.tree.type,'AccountCreatedDemoScreen');assert(nodes(f.tree,'TextInput').every(n=>n.props.value===''));assert(!f.text().includes('Passwords do not match.'));assert.equal(f.clears.length,6);f.unmount();
    t=a.tree; if(i%2){a.env.back();}else{const done=t.props.onContinue;done();done();}t=a.tree;assert.equal(t.type,'LoginScreen');
    registerProps.onDemoValidated();assert.equal(a.tree.type,'LoginScreen','stale transition ignored');
    const fresh=mount('RegisterScreen',{onLogin(){}});assert(nodes(fresh.tree,'TextInput').every(n=>n.props.value===''));assert(!fresh.text().includes('Passwords do not match.'));fresh.unmount();
    const loginProps=t.props;
    const login=mount('LoginScreen',loginProps);login.submit();assert.equal(a.tree.type,'LoginScreen');assert(login.focuses.includes(0));
    login.set(0,'sample identifier');login.set(1,'   ');login.submit();assert.equal(a.tree.type,'LoginScreen');assert(login.focuses.includes(1));
    login.set(1,'sample password');const replay=login.submit();replay();
    t=a.tree;assert.equal(t.type,'ChatHomeDemoScreen');assert(nodes(login.tree,'TextInput').every(n=>n.props.value===''));assert(nodes(login.tree,'TextInput')[1].props.secureTextEntry);assert.equal(login.clears.length,2);
    assert(!JSON.stringify(t.props).includes('sample password'));login.unmount();replay();
    const oldChat=t.props;
    const chat=chatFixture(400,772,1,{mode:'demo',onExit:oldChat.onExit});chat.set('Local draft');
    nodes(chat.tree,'Pressable').find(n=>n.props.accessibilityLabel==='How do I renew my NIC?').props.onPress();chat.draw();
    assert.equal(chat.rt.timers.size,1);nodes(chat.tree,'Pressable').find(n=>n.props.accessibilityLabel==='Back to Chat Home').props.onPress();chat.draw();
    assert.equal(chat.rt.timers.size,0);assert.equal(nodes(chat.tree,'TextInput')[0].props.value,'Local draft');
    nodes(chat.tree,'Pressable').find(n=>n.props.accessibilityLabel==='Open menu').props.onPress();chat.draw();
    const logout=nodes(chat.tree,'Pressable').find(n=>n.props.accessibilityLabel==='Logout').props.onPress;
    logout();logout();chat.draw();assert.equal(chat.exits,1);assert.equal(nodes(chat.tree,'TextInput')[0].props.value,'');chat.unmount();
    t=a.tree;assert.equal(t.type,'LoginScreen');const loggedOut=t.props;
    const emptyLogin=mount('LoginScreen',loggedOut);assert(nodes(emptyLogin.tree,'TextInput').every(n=>n.props.value===''));emptyLogin.unmount();
    a.env.back();assert.equal(a.tree.type,origins[i],'Back after Logout preserves original origin');
    // Re-enter Login from the same origin; no route history may reopen Chat Home.
    a.tree.props.onLogin();
    t=a.tree;assert.equal(t.type,'LoginScreen');loginProps.onDemoValidated();assert.equal(a.tree.type,'LoginScreen','old Login visit cannot navigate a new visit');
    t.props.onDemoValidated();t=a.tree;assert.equal(t.type,'ChatHomeDemoScreen');logout();oldChat.onExit();assert.equal(a.tree.type,'ChatHomeDemoScreen','old Chat/Logout cannot exit a later visit');t.props.onExit();t=a.tree;
    const freshLogin=mount('LoginScreen',t.props);assert(nodes(freshLogin.tree,'TextInput').every(n=>n.props.value===''));freshLogin.unmount();
    t.props.onForgotPassword();t=a.tree;
    for(const method of ['onBack','onDifferentNic','androidBack']){
      const props=t.props,g=mount('ForgotPasswordScreen',props);g.set(0,'   ');g.submit();assert.equal(a.tree.type,'ForgotPasswordScreen');
      g.set(0,' unvalidated NIC ');const twice=g.submit();twice();assert.equal(a.tree.type,'CheckEmailDemoScreen');assert.equal(nodes(g.tree,'TextInput')[0].props.value,'');assert(!g.text().includes('Enter your National ID number.'));g.unmount();
      t=a.tree;if(method==='androidBack')a.env.back();else t.props[method]();t=a.tree;assert.equal(t.type,'ForgotPasswordScreen');
      const clean=mount('ForgotPasswordScreen',t.props);assert.equal(nodes(clean.tree,'TextInput')[0].props.value,'');assert(!clean.text().includes('Enter your National ID number.'));clean.unmount();
    }
    a.env.visible=true;a.env.back();assert.equal(a.tree.type,'ForgotPasswordScreen');assert(!a.env.visible);a.env.back();t=a.tree;assert.equal(t.type,'LoginScreen');t.props.onBack();assert.equal(a.tree.type,origins[i]);a.rt.unmount();
  }
  // Existing standalone previews retain priority when a user combines flags.
  for(const [key,name]of keys.slice(1).map((k,i)=>[k,['AccountCreatedPreviewScreen','CheckEmailPreviewScreen','ResetPasswordPreviewScreen','PasswordUpdatedPreviewScreen'][i]])){
    flags(true,'1');process.env[key]='1';const a=app();assert.equal(a.tree.type,name);a.rt.unmount();
  }
  flags(true,'1');
  for(const [w,h] of [[320,568],[360,640],[360,800],[412,915],[600,960],[400,772]])for(const scale of [1,2,3]){
    for(const [name,file,props] of [['AccountCreatedDemoScreen','AccountCreatedPreviewScreen',{onContinue(){}}],['CheckEmailDemoScreen','CheckEmailPreviewScreen',{onBack(){},onDifferentNic(){}}]]){
      const rt=runtime(),env=environment(w,h,scale),component=loader(rt,env)(file+'.tsx')[name];let tree;
      const draw=()=>{tree=rt.render(()=>unwrap(component(props)));rt.effects();};draw();
      function measure(){for(const n of nodes(tree))if(n.props.onLayout){const s=flat(n.props.style);const height=n.type==='Text'?18*scale:s.borderColor==='#f5a915'?72*scale*scale:s.width===86?47*scale:s.paddingVertical?383*scale:h-80;n.props.onLayout({nativeEvent:{layout:{width:Math.min(480,w-32),height}}});}draw();}
      measure();const writes=rt.writes;measure();measure();assert.equal(rt.writes,writes);
      const texts=nodes(tree,'Text').map(n=>n.props.children);
      if(name==='AccountCreatedDemoScreen'){
        assert(texts.includes('UI demo \u2014 no account was created'));assert(!texts.includes('Exit preview'));
        const actions=nodes(tree,'Pressable');assert.equal(actions.length,1);assert(!actions[0].props.disabled);assert.equal(typeof actions[0].props.onPress,'function');
      }else{
        assert(texts.includes('Preview \u2014 no email was sent'));assert(texts.includes('sample@example.test'));
        const disabled=nodes(tree,'Pressable').filter(n=>n.props.disabled);assert.equal(disabled.length,1);assert(!disabled[0].props.onPress);
        assert(nodes(tree,'Text').some(n=>n.props.accessibilityState?.disabled&&!n.props.onPress));
      }
      assert.equal(nodes(tree,'TextInput').length,0);rt.unmount();
    }
  }
  const launchSource=fs.readFileSync(path.join(root,'scripts/demo-connected-ui.cjs'),'utf8');
  for(const args of [[],['--port','8095']]){
    let launch,exit;const parent=Object.fromEntries(keys.map(k=>[k,'1']));parent[keys[0]]='0';
    vm.runInNewContext(launchSource,{__dirname:path.join(root,'scripts'),require:n=>n==='node:path'?path:{spawnSync:(exe,args,options)=>(launch={args,options},{status:7})},process:{argv:['node','demo',...args],execPath:process.execPath,env:parent,exit:s=>{exit=s;}},console:{log(){},error(){}}});
    assert.deepEqual(Array.from(launch.args).slice(1),['start','--go','--lan',...args]);assert.equal(exit,7);assert.equal(parent[keys[0]],'0');assert.equal(launch.options.env[keys[0]],'1');keys.slice(1).forEach(k=>assert.equal(launch.options.env[k],'0'));
  }
  for(const args of [['--dev'],['--port','0'],['--port','65536'],['--port','abc']])assert.throws(()=>vm.runInNewContext(launchSource,{require:n=>n==='node:path'?path:{},process:{argv:['node','demo',...args]},console}),/Optional supported argument/);
  if(process.argv.includes('--export')){
    const dir=path.resolve(root,process.argv[process.argv.indexOf('--export')+1]||'dist/android');const files=[];
    function walk(d){for(const e of fs.readdirSync(d,{withFileTypes:true})){const f=path.join(d,e.name);e.isDirectory()?walk(f):files.push(f);}}walk(dir);
    const bundles=files.filter(f=>/\.(js|hbc)$/.test(f));assert(bundles.length);
    for(const f of bundles){const bytes=fs.readFileSync(f);for(const marker of ['ChatHomeDemoScreen','ChatHomeContent','UI demo only. Credentials are not verified; valid fields open Chat Home.','Log In. Open Chat Home in UI demo only; credentials are not verified.','UI demo \u00b7 Credentials were not verified. AI and actions are disconnected.','AccountCreatedDemoScreen','CheckEmailDemoScreen','UI demo \u2014 no account was created','Preview \u2014 no email was sent','sample@example.test','Rathishan'])for(const encoding of ['utf8','utf16le'])assert(!bytes.includes(Buffer.from(marker,encoding)),marker+' in '+f);}
    for(const icon of ['assets/figma/account-created/check-circle.png','assets/figma/check-email/mail.png']){const hash=crypto.createHash('sha256').update(fs.readFileSync(path.join(root,icon))).digest('hex');assert(!files.some(f=>crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex')===hash));}
    console.log('PASS connected demo excluded from production artifact:',dir);
  }
  console.log('PASS connected demo: 8 mode/flag cases, all three actual form submissions, validation/cleanup/repeated events, all 6 Login origins, keyboard-first Back, Login visit guards, Chat Back, confirmation Back/Continue, fresh recovery forms, standalone precedence, 36 synthetic confirmation layouts, launcher args/environment. No native/device rendering.');
} finally {
  global.__DEV__=savedDev;keys.forEach((k,i)=>saved[i]===undefined?delete process.env[k]:process.env[k]=saved[i]);
}
