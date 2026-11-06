const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),ts=require('typescript');
const modules={};
function load(name){
 if(modules[name])return modules[name].exports;
 const source=ts.transpileModule(fs.readFileSync(path.resolve(__dirname,'../../services',name+'.ts'),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;
 const module={exports:{}};modules[name]=module;
 new Function('require','module','exports',source)(id=>load(id.replace('./','')),module,module.exports);return module.exports;
}
const {createNativeAuthApi,NativeAuthError}=load('nativeAuthApi');
const {createNativeSession}=load('nativeSession');
const issued=(id='one')=>({accessToken:'synthetic-access-'+id,refreshToken:'synthetic-refresh-'+id,expiresIn:300,refreshExpiresIn:1800});
(async()=>{
 let calls=[];
 const api=createNativeAuthApi('https://api.example.test',async(url,options)=>{
  calls.push({url,options});
  assert.equal(options.credentials,'omit');assert.equal(options.redirect,'error');assert.deepEqual(Object.keys(options.headers),['Content-Type']);
  if(url.endsWith('/revoke'))return new Response(null,{status:204});
  return Response.json({access_token:'synthetic-access',refresh_token:'synthetic-refresh',token_type:'Bearer',expires_in:300,refresh_expires_in:1800});
 });
 await api.login(' test@example.test ',' password unchanged ');
 assert.deepEqual(JSON.parse(calls[0].options.body),{identifier:'test@example.test',password:' password unchanged '});
 await api.refresh('synthetic-refresh');assert.deepEqual(JSON.parse(calls[1].options.body),{refresh_token:'synthetic-refresh'});
 await api.revoke('synthetic-refresh');assert.equal(calls.length,3);
 for(const status of [401,429,503]){
  const bad=createNativeAuthApi('https://api.example.test',async()=>new Response('private provider detail',{status}));
  await assert.rejects(bad.login('test@example.test','x'),e=>e instanceof NativeAuthError&&e.status===status&&!e.message.includes('private provider'));
 }
 const malformed=createNativeAuthApi('https://api.example.test',async()=>Response.json({access_token:'x'}));await assert.rejects(malformed.login('test@example.test','x'),e=>e.status===502);
 const network=createNativeAuthApi('https://api.example.test',async()=>{throw Error('private failure')});await assert.rejects(network.refresh('x'),e=>e.status===503&&!e.message.includes('private failure'));
 let saved=null,now=1000,events=[],refreshes=0,revokes=0;
 const store={load:async()=>saved,save:async value=>{events.push('save');saved=value;},clear:async()=>{events.push('clear');saved=null;}};
 let failRefresh=false;
 const provider={login:async()=>issued(),refresh:async token=>{events.push('refresh');refreshes++;assert.equal(saved,null,'old persisted credential removed before network');if(failRefresh)throw new NativeAuthError(503);return issued('rotated');},revoke:async()=>{revokes++;}};
 const session=createNativeSession(provider,store,()=>now);
 assert.equal(await session.restore(),false);
 await session.login('test@example.test','synthetic password');assert.equal(saved.token,'synthetic-refresh-one');assert.equal(Object.hasOwn(saved,'accessToken'),false);
 assert.equal(await session.withAccess(async token=>token),'synthetic-access-one');
 await assert.rejects(session.login('other@example.test','x'),/Sign out/);
 now+=300000;events=[];
 await Promise.all([session.withAccess(async()=>true),session.withAccess(async()=>true)]);
 assert.equal(refreshes,1);assert.deepEqual(events,['clear','refresh','save']);
 await session.logout();assert.equal(saved,null);assert.equal(revokes,1);assert.equal(await session.restore(),false);
 saved={token:'synthetic-refresh',expiresAt:now+600000};failRefresh=true;
 await assert.rejects(session.restore(),e=>e.status===503);assert.equal(saved,null);const failures=refreshes;assert.equal(await session.restore(),false);assert.equal(refreshes,failures,'unknown refresh outcome not replayed');
 const broken={...store,save:async()=>{throw Error('private storage')}};
 const failStore=createNativeSession(provider,broken,()=>now);await assert.rejects(failStore.login('test@example.test','x'),/Unable to save/);assert.equal(await failStore.restore(),false);
 assert.equal(saved,null);
 console.log('PASS: native request contract, no browser headers/cookies, safe failures, memory-only access, secure refresh persistence, serialized rotation, consume-before-refresh, no ambiguous replay, logout and failed-storage revocation.');
})().catch(error=>{console.error(error);process.exitCode=1});
