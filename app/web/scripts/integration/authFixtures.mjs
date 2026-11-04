import { origin } from './browserHarness.mjs'
// Synthetic HTTP responses only. No real backend accounts, mail or passwords.
export async function installAuthFixtures(socket, send) {
  const model={chatStatus:503,unsafeChat:false,session:false,failLogin:false,registrationStatus:202,recoveryStatus:202,resetStatus:204,maskedEmail:'te***@example.test',calls:[],errors:[],used:false}
  socket.addEventListener('message',({data})=>{
    const event=JSON.parse(data)
    if(event.method!=='Fetch.requestPaused')return
    void handle(event.params).catch(()=>{model.errors.push('Fixture fulfillment failed')})
  })
  async function handle({requestId,request}) {
    const path=new URL(request.url).pathname,method=request.method
    const headers=[{name:'Access-Control-Allow-Origin',value:origin},{name:'Access-Control-Allow-Credentials',value:'true'},{name:'Access-Control-Allow-Headers',value:'Content-Type, Idempotency-Key, X-GovAI-Web, X-CSRF-Token'},{name:'Access-Control-Allow-Methods',value:'GET, POST'},{name:'Content-Type',value:'application/json'},{name:'Cache-Control',value:'no-store'}]
    let code=200,result={}
    if(method==='OPTIONS')code=204
    else {
      const body=request.postData?JSON.parse(request.postData):undefined
      // Keep only shape/transport evidence; never retain submitted credentials.
      model.calls.push({path,method,keys:body?Object.keys(body).sort():[],headers:Object.keys(request.headers).map(x=>x.toLowerCase())})
      if(path.endsWith('/web/login')) {
        if(model.failLogin){code=401;result={code:'session_rejected'}}
        else{model.session=true;result={csrf_token:'A'.repeat(43),expires_at:'2099-01-01T00:00:00Z'};headers.push({name:'Set-Cookie',value:'__Host-govai_session='+'B'.repeat(43)+'; Path=/; Secure; HttpOnly; SameSite=Strict'})}
      } else if(path.endsWith('/web/session')) {code=model.session?200:401;result=model.session?{subject:'synthetic-subject'}:{code:'session_rejected'}}
      else if(path.endsWith('/web/csrf')){code=model.session?200:401;result=model.session?{csrf_token:'A'.repeat(43)}:{code:'session_rejected'}}
      else if(path.endsWith('/web/refresh')){code=model.session?204:401;result={code:'session_rejected'}}
      else if(path.endsWith('/web/logout')){model.session=false;code=204;headers.push({name:'Set-Cookie',value:'__Host-govai_session=; Path=/; Secure; HttpOnly; SameSite=Strict; Max-Age=0'})}
      else if(path.endsWith('/web/chat')){code=model.chatStatus;result=code===200?{answer:'Synthetic <img src=x onerror=alert(1)> answer',sources:[{title:'Example reference',url:model.unsafeChat?'javascript:bad':'https://example.test/guide'}]}:{code:'chat_unavailable'}}
      else if(path.endsWith('/registrations')){code=model.registrationStatus;result=code===202?{status:'received'}:{code:'registration_unavailable'}}
      else if(path.endsWith('/forgot-password')){code=model.recoveryStatus;result=code===202?{status:'received',...(model.maskedEmail?{masked_email:model.maskedEmail}:{})}:{code:'recovery_unavailable'}}
      else if(path.endsWith('/reset-password')){code=model.used?400:model.resetStatus;if(code===204)model.used=true;result={code:'invalid_or_expired_reset'}}
      else{code=503;result={code:'unexpected_fixture_endpoint'};model.errors.push('Unexpected API route')}
    }
    await send('Fetch.fulfillRequest',{requestId,responseCode:code,responseHeaders:headers,body:code===204?'':Buffer.from(JSON.stringify(result)).toString('base64')})
  }
  await send('Fetch.enable',{patterns:[{urlPattern:'https://localhost:5001/api/*'}]})
  return model
}
