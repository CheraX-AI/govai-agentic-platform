const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
// Compile only the two pure helpers in memory; never import native Expo modules.
const modules = {};
function load(name) {
  if (modules[name]) return modules[name].exports;
  const file = path.resolve(__dirname, '../../services', name + '.ts');
  const source = ts.transpileModule(fs.readFileSync(file, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const module = { exports: {} }; modules[name] = module;
  new Function('require','module','exports', source)(id => load(id.replace('./','')), module, module.exports);
  return module.exports;
}
const { validateApiBase } = load('apiConfig');
const { createRefreshStorage } = load('refreshStorage');
(async () => {
  for (const value of [undefined, '', 'http://192.168.1.2:5001', 'https://localhost:5001', 'https://127.0.0.1:5001', 'https://user:secret@api.example.test', 'https://api.example.test/path', 'https://api.example.test/?secret=x']) assert.throws(() => validateApiBase(value));
  assert.equal(validateApiBase('https://api.example.test/'), 'https://api.example.test');
  let saved = null, now = 100;
  const storage = { getItemAsync: async () => saved, setItemAsync: async (_key,value) => { saved=value; }, deleteItemAsync: async () => { saved=null; } };
  const store = createRefreshStorage('https://api.example.test', storage, () => now);
  assert.equal(await store.load(), null);
  await store.save({token:'synthetic-refresh', expiresAt:200});
  assert.deepEqual(await store.load(), {token:'synthetic-refresh', expiresAt:200});
  assert.equal(await createRefreshStorage('https://other.example.test',storage,()=>now).load(),null);
  assert.equal(saved.includes('password'),false); assert.equal(saved.includes('accessToken'),false);
  now=200; assert.equal(await store.load(),null); assert.equal(saved,null);
  saved='bad JSON'; assert.equal(await store.load(),null);
  saved=JSON.stringify({version:1,apiBase:'https://api.example.test',token:'x',expiresAt:300,password:'not allowed'});assert.equal(await store.load(),null);
  await assert.rejects(store.save({token:'invalid token',expiresAt:300}));
  const unavailable=createRefreshStorage('https://api.example.test',{getItemAsync:async()=>{throw Error('private failure')},setItemAsync:async()=>{throw Error('private failure')},deleteItemAsync:async()=>{throw Error('private failure')}});
  await assert.rejects(unavailable.load(),e=>!e.message.includes('private failure'));
  await store.clear(); assert.equal(saved,null);
  console.log('PASS: phone HTTPS config, secure-storage boundary, environment binding, expiry/malformed cleanup and safe errors. Native SecureStore and real phone not tested.');
})().catch(error=>{console.error(error);process.exitCode=1});
