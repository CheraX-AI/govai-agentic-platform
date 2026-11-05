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

const {sendChat} = load('chatApi');
const {requestRecovery,resetPassword,parseResetLink} = load('recoveryApi');
const {utf8Length,apiRequest}=load('publicRequest');
const signal=new AbortController().signal;
const base='https://api.example.test';
const resetToken='a'.repeat(43);
(async()=>{
 assert.equal(utf8Length('Tamil ????? ??'),Buffer.byteLength('Tamil ????? ??'));
 let request;
 const reply=await sendChat(base,'synthetic-access','  Passport question  ',signal,async(url,options)=>{
  request=options;
  assert.equal(url,base+'/api/chat/message');
  return new Response(JSON.stringify({answer:'See [1].',sources:[{number:1,title:'Passport guide',page:2,url:'https://government.example.test/guide'}]}));
 });
 assert.equal(reply.sources[0].page,2);
 assert.equal(request.headers.Authorization,'Bearer synthetic-access');
 assert.equal(request.credentials,'omit');
 assert.equal(request.headers.Cookie,undefined); assert.equal(request.headers.Origin,undefined);
 assert.deepEqual(JSON.parse(request.body),{message:'Passport question'});
 for(const status of [401,403,429,500,504]) await assert.rejects(sendChat(base,'access','Question',signal,async()=>new Response('private failure',{status})),e=>e.status===status&&!e.message.includes('private failure'));
 for(const body of [
  {answer:'',sources:[]},{answer:'Answer',sources:[{number:1,title:'x',url:'javascript:alert(1)'}]},
  {answer:'Answer',sources:[{number:1,title:'x'},{number:1,title:'y'}]},
  {answer:'Answer',sources:[{number:1,title:'x',page:0}]},
 ]) await assert.rejects(sendChat(base,'access','Question',signal,async()=>new Response(JSON.stringify(body))));
 let called=false;
 await assert.rejects(sendChat(base,'access',' ',signal,async()=>{called=true;})); assert.equal(called,false);
 const controller=new AbortController();controller.abort();
 await assert.rejects(sendChat(base,'access','Question',controller.signal,async(_url,options)=>{assert.equal(options.signal.aborted,true);throw Error('private');}));
 await assert.rejects(apiRequest(base,'/test',{}, {timeout:5}, async(_url,options)=>new Promise((_resolve,reject)=>options.signal.addEventListener('abort',()=>reject(Error('timeout'))))));
 const receipt=await requestRecovery(base,'991234567v',async(url,options)=>{
  assert.equal(url,base+'/api/v1/auth/forgot-password');assert.deepEqual(JSON.parse(options.body),{nic:'991234567V'});
  assert.equal(options.headers.Authorization,undefined);
  return new Response(JSON.stringify({status:'received',masked_email:'a***@example.test'}),{status:202});
 });assert.equal(receipt.maskedEmail,'a***@example.test');
 for(const status of [400,429,503])await assert.rejects(requestRecovery(base,'991234567V',async()=>new Response('private',{status})),e=>!e.message.includes('private'));
 await assert.rejects(requestRecovery(base,'invalid',async()=>{throw Error('should not call')}));
 await resetPassword(base,resetToken,' exact password ',' exact password ',async(url,options)=>{
  assert.equal(url,base+'/api/v1/auth/reset-password');assert.equal(JSON.parse(options.body).new_password,' exact password ');
  assert.equal(options.headers.Authorization,undefined);return new Response(null,{status:204});
 });
 for(const status of [400,422,429,500])await assert.rejects(resetPassword(base,resetToken,'password','password',async()=>new Response('private',{status})));
 await assert.rejects(resetPassword(base,resetToken,'short','short'));
 await assert.rejects(resetPassword(base,'invalid','password','password'));
 assert.equal(parseResetLink('https://app.example.test/reset-password#token='+resetToken,'https://app.example.test/reset-password'),resetToken);
 assert.equal(parseResetLink('https://evil.test/reset-password#token='+resetToken,'https://app.example.test/reset-password'),null);
 assert.equal(parseResetLink('govai://reset-password#token='+resetToken),resetToken);
 assert.equal(parseResetLink('exp://192.0.2.1:8081/--/reset-password#token='+resetToken),resetToken);
 for(const value of ['https://evil.test/reset-password#token='+resetToken,'govai://other#token='+resetToken,'govai://reset-password?token='+resetToken,'govai://reset-password#token=invalid'])assert.equal(parseResetLink(value),null);
 console.log('PASS: bearer chat, source validation, cancellation/deadlines, recovery/reset contracts, safe errors and dedicated reset links.');
})().catch(e=>{console.error(e);process.exitCode=1});
