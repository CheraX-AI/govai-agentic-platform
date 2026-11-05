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
const { registerCitizen } = load('registrationApi');
(async () => {
  const payload = {full_name:'Synthetic Citizen',nic:'991234567V',mobile_number:'+94771234567',email:'synthetic@example.test',password:' exact password '};
  const key='12345678-1234-4234-8234-123456789abc';
  let sent;
  await registerCitizen('https://api.example.test',payload,key,async(url,options)=>{
    sent=options;
    assert.equal(url,'https://api.example.test/api/v1/registrations');
    return new Response(JSON.stringify({status:'received'}),{status:202});
  });
  assert.equal(sent.credentials,'omit');
  assert.equal(sent.headers['Idempotency-Key'],key);
  assert.equal(JSON.parse(sent.body).password,payload.password);
  assert.equal('Origin' in sent.headers,false);
  for (const status of [400,422,429,500]) {
    await assert.rejects(registerCitizen('https://api.example.test',payload,key,async()=>new Response('private upstream details',{status})),error=>!error.message.includes('private upstream'));
  }
  await assert.rejects(registerCitizen('https://api.example.test',payload,key,async()=>new Response('{}',{status:202})));
  await assert.rejects(registerCitizen('https://api.example.test',payload,key,async()=>{throw Error('private network details')}),error=>!error.message.includes('private network'));
  let called=false;
  await assert.rejects(registerCitizen('',payload,key,async()=>{called=true;}));
  assert.equal(called,false);
  console.log('PASS: registration contract, idempotency, exact password, safe errors and missing configuration.');
})().catch(error=>{console.error(error);process.exitCode=1});
