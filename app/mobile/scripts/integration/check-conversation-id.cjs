// Conversation ID handling in the mobile chat API. Run: node scripts/integration/check-conversation-id.cjs
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
// Compile only the pure service modules in memory; never import native Expo modules.
const modules = {};
function load(name) {
  if (modules[name]) return modules[name].exports;
  const file = path.resolve(__dirname, '../../services', name + '.ts');
  const source = ts.transpileModule(fs.readFileSync(file, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const module = { exports: {} }; modules[name] = module;
  new Function('require','module','exports', source)(id => load(id.replace('./','')), module, module.exports);
  return module.exports;
}

const { sendChat, isConversationId } = load('chatApi');
const base = 'https://api.example.test';
const signal = new AbortController().signal;
const first = '3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34';
const other = '01936f3e-7b2a-7c4d-9e5f-0a1b2c3d4e5f';

// A fake core-service that records each body and answers with the given reply
function core(reply, status = 200) {
  const bodies = [];
  const transport = async (_url, options) => { bodies.push(JSON.parse(options.body)); return new Response(JSON.stringify(reply), { status }); };
  return { bodies, transport };
}

(async () => {
  // First message omits the ID; the backend's ID comes back
  let fake = core({ answer: 'Synthetic answer', sources: [], conversation_id: first });
  let reply = await sendChat(base, 'synthetic-access', ' Passport question ', signal, fake.transport);
  assert.deepEqual(fake.bodies[0], { message: 'Passport question' });
  assert.equal(reply.conversationId, first);

  // A later message in the same thread reuses the ID
  reply = await sendChat(base, 'synthetic-access', 'Follow-up', signal, fake.transport, first);
  assert.deepEqual(fake.bodies[1], { message: 'Follow-up', conversation_id: first });
  assert.equal(reply.conversationId, first);

  // Legacy backend without conversation_id still works
  fake = core({ answer: 'Synthetic answer', sources: [] });
  reply = await sendChat(base, 'synthetic-access', 'Question', signal, fake.transport);
  assert.equal(reply.conversationId, undefined);

  // Returned IDs are validated, and a continued thread may not switch conversations
  for (const conversation_id of ['not-a-uuid', first.toUpperCase(), null, 7, first + '\n']) {
    await assert.rejects(sendChat(base, 'access', 'Question', signal, core({ answer: 'x', sources: [], conversation_id }).transport), e => e.status === 502);
  }
  await assert.rejects(sendChat(base, 'access', 'Question', signal, core({ answer: 'x', sources: [], conversation_id: other }).transport, first), e => e.status === 502);
  await assert.rejects(sendChat(base, 'access', 'Question', signal, core({ answer: 'x', sources: [] }).transport, first), e => e.status === 502);

  // A malformed local ID is never sent
  let called = false;
  await assert.rejects(sendChat(base, 'access', 'Question', signal, async () => { called = true; }, 'not-a-uuid'), e => e.status === 400);
  assert.equal(called, false);

  // Unknown or another citizen's conversation keeps its 404 status for the chat screen
  await assert.rejects(sendChat(base, 'access', 'Question', signal, core({ error: 'private detail', code: 'conversation_not_found' }, 404).transport, first), e => e.status === 404 && !e.message.includes('private'));

  assert.equal(isConversationId(first), true);
  assert.equal(isConversationId('00000000-0000-0000-0000-000000000000'), false);
  console.log('PASS: mobile chat omits conversation_id on the first message, reuses it after, validates returned IDs and keeps 404 for unknown conversations.');
})().catch(e => { console.error(e); process.exitCode = 1; });
