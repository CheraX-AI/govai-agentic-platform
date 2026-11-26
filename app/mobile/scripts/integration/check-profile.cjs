const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const root = path.resolve(__dirname, '../..');
function loader(stubs = {}) {
  const modules = {};
  function load(file) {
    file = path.resolve(root, file);
    if (!path.extname(file)) file += '.ts';
    if (modules[file]) return modules[file].exports;
    const module = { exports: {} }; modules[file] = module;
    const source = ts.transpileModule(fs.readFileSync(file, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
    new Function('require', 'module', 'exports', source)(id => {
      if (Object.hasOwn(stubs, id)) return stubs[id];
      return load(path.resolve(path.dirname(file), id));
    }, module, module.exports);
    return module.exports;
  }
  return load;
}
const load = loader();
const { createProfileApi, ProfileError, MAX_PROFILE_PHOTO_BYTES, validateProfile, photoContentType } = load('services/profileApi');
const base = 'https://api.example.test';
const fields = { name_si: '', name_ta: '', date_of_birth: '2003-08-15', gender: 'male', address: { line1: 'Synthetic road', line2: '', city: 'Colombo' } };
const initial = { ...fields, full_name: 'Synthetic Citizen', nic: '200322812345', mobile_number: '+94771234567', email: null, revision: 0, has_photo: false, photo_url: null };
const png = Uint8Array.from(Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aX1sAAAAASUVORK5CYII=', 'base64'));
const image = { uri: 'file:///synthetic.png', data: png.buffer, contentType: 'image/png' };
const access = action => action('synthetic-access');
(async () => {
  let count = 0;
  const api = createProfileApi(() => base, access, async (url, options) => {
    count++;
    assert.equal(options.credentials, 'omit'); assert.equal(options.cache, 'no-store'); assert.equal(options.redirect, 'error');
    assert.equal(options.headers.Authorization, 'Bearer synthetic-access');
    for (const name of ['Cookie', 'Origin', 'X-GovAI-Web', 'X-CSRF-Token']) assert.equal(options.headers[name], undefined);
    if (options.method === 'PUT' && url.endsWith('/photo')) {
      assert.equal(options.headers['If-Match'], '"0"'); assert.equal(options.headers['Content-Type'], 'image/png'); assert.equal(options.body, image.data);
      return Response.json({ revision: 1, has_photo: true, photo_url: '/api/v1/profile/photo' });
    }
    if (options.method === 'PUT') {
      assert.equal(url, base + '/api/v1/profile');
      assert.deepEqual(JSON.parse(options.body), { revision: 0, ...fields });
      return Response.json({ ...initial, revision: 1 });
    }
    if (url.endsWith('/photo')) return new Response(png, { headers: { 'Content-Type': 'image/png' } });
    assert.equal(url, base + '/api/v1/profile');
    return Response.json({ ...initial, subject: 'must-not-be-exposed' });
  });
  assert.deepEqual(await api.load(), initial);
  await api.save({ ...fields, nic: 'FORGED', subject: 'other', address: { ...fields.address, owner: 'other' } }, 0);
  await api.upload(image, 0);
  assert.equal(await api.photo(), 'data:image/png;base64,' + Buffer.from(png).toString('base64'));
  assert.equal(count, 4);
  for (const status of [401, 403, 404, 409, 413, 415, 422, 428, 429, 503]) {
    let attempts = 0;
    const broken = createProfileApi(() => base, access, async () => { attempts++; return Response.json({ code: 'dob_nic_mismatch', error: 'private diagnostics' }, { status }); });
    await assert.rejects(broken.save(fields, 0), error => error.status === status && !error.message.includes('private'));
    assert.equal(attempts, 1, 'A failed write is never replayed');
  }
  for (const invalid of [{ ...initial, revision: -1 }, { ...initial, photo_url: 'https://other.test/private' }, { ...initial, address: null }, { ...initial, gender: 'other' }]) {
    const malformed = createProfileApi(() => base, access, async () => Response.json(invalid));
    await assert.rejects(malformed.load(), error => error.status === 502);
  }
  const network = createProfileApi(() => base, access, async () => { throw Error('private network detail'); });
  await assert.rejects(network.save(fields, 0), error => error.status === 503 && !error.message.includes('private'));
  assert.equal(validateProfile(fields), '');
  for (const date of ['2003-02-29', '2003-13-01', '9999-01-01', '15/08/2003']) assert.notEqual(validateProfile({ ...fields, date_of_birth: date }), '');
  assert.equal(validateProfile({ ...fields, name_si: '\u{1F600}'.repeat(200) }), '');
  assert.notEqual(validateProfile({ ...fields, address: { ...fields.address, city: 'x'.repeat(101) } }), '');
  assert.throws(() => photoContentType(new Uint8Array(MAX_PROFILE_PHOTO_BYTES + 1)), error => error.status === 413);
  assert.throws(() => photoContentType(new Uint8Array([1, 2, 3])), error => error.status === 415);
  const boundary = new Uint8Array(MAX_PROFILE_PHOTO_BYTES); boundary.set(png.slice(0, 8)); assert.equal(photoContentType(boundary), 'image/png');
  let requestSignal;
  const cancel = new AbortController();
  const pending = createProfileApi(() => base, access, async (_url, options) => {
    requestSignal = options.signal;
    return { ok: true, text: () => new Promise((_resolve, reject) => options.signal.addEventListener('abort', () => reject(Error('cancelled')))) };
  }).load(cancel.signal);
  await new Promise(resolve => setImmediate(resolve)); cancel.abort();
  await assert.rejects(pending); assert.equal(requestSignal.aborted, true);

  // Test real hook transitions in an in-memory React lifecycle harness, not a device.
  let chosen = image;
  const driver = makeDriver();
  const hookLoad = loader({ react: driver.react, '../services/profilePhotoPicker': { pickProfilePhoto: async () => chosen } });
  const { useCitizenProfile } = hookLoad('auth/useCitizenProfile');
  const { ProfileError: HookError } = hookLoad('services/profileApi');
  let stored = structuredClone(initial), writes = 0, fail = 0;
  const service = {
    load: async () => structuredClone(stored),
    save: async (details, revision) => {
      writes++;
      if (fail) throw new HookError(fail, 'Synthetic failure');
      assert.equal(revision, stored.revision);
      stored = { ...stored, ...details, address: { ...details.address }, revision: revision + 1 };
      return structuredClone(stored);
    },
    photo: async () => 'data:image/png;base64,' + Buffer.from(png).toString('base64'),
    upload: async (_photo, revision) => { assert.equal(revision, stored.revision); stored = { ...stored, revision: revision + 1, has_photo: true, photo_url: '/api/v1/profile/photo' }; return { revision: stored.revision, has_photo: true, photo_url: stored.photo_url }; },
  };
  driver.mount(() => useCitizenProfile(service)); await driver.flush();
  assert.equal(driver.current.profile.full_name, 'Synthetic Citizen');
  driver.current.changeAddress('city', 'Kandy'); await driver.flush();
  assert.equal(driver.current.dirty, true);
  await Promise.all([driver.current.save(), driver.current.save()]); await driver.flush();
  assert.equal(writes, 1); assert.equal(driver.current.dirty, false); assert.equal(stored.address.city, 'Kandy');
  driver.current.changeAddress('city', 'Galle'); await driver.flush();
  stored.revision++; fail = 409;
  await driver.current.save(); await driver.flush();
  assert.equal(driver.current.needsReload, true); assert.equal(driver.current.draft.address.city, 'Galle');
  const before = writes; await driver.current.save(); assert.equal(writes, before);
  await driver.current.reload(true); await driver.flush();
  assert.equal(driver.current.needsReload, false); assert.equal(driver.current.draft.address.city, 'Galle');
  fail = 0; await driver.current.save(); await driver.flush(); assert.equal(stored.address.city, 'Galle');
  driver.current.changeAddress('city', 'Matara'); await driver.flush();
  fail = 503; await driver.current.save(); await driver.flush(); assert.equal(driver.current.needsReload, true);
  await driver.current.reload(false); await driver.flush(); assert.equal(driver.current.draft.address.city, 'Galle');
  fail = 0; driver.current.change('name_si', 'Synthetic local name'); await driver.flush();
  await driver.current.choosePhoto(); await driver.flush(); assert.ok(driver.current.selectedPhoto);
  await driver.current.uploadPhoto(); await driver.flush(); assert.equal(driver.current.dirty, true); assert.equal(driver.current.profile.revision, stored.revision);
  await driver.current.save(); await driver.flush(); assert.equal(stored.name_si, 'Synthetic local name');
  chosen = null; await driver.current.choosePhoto(); await driver.flush(); assert.equal(driver.current.selectedPhoto, null);
  driver.unmount();
  console.log('PASS: native profile bearer/no-cookie transport, field allowlist, raw photo revisions/size, authenticated photo decoding, malformed responses, safe errors/cancellation, and hook load/save/duplicate/conflict/uncertain-result/photo reconciliation. Synthetic tests; no phone/live integration claim.');
})().catch(error => { console.error(error); process.exitCode = 1; });

function makeDriver() {
  const slots = []; let index = 0, render, scheduled = false, alive = true, current; const effects = [];
  function slot(initial) { const i = index++; if (!slots[i]) slots[i] = initial(); return slots[i]; }
  function schedule() { if (!alive || scheduled) return; scheduled = true; queueMicrotask(() => { scheduled = false; if (alive) run(); }); }
  function run() { index = 0; current = render(); for (const action of effects.splice(0)) action(); }
  const react = {
    useState(initial) { const state = slot(() => ({ value: typeof initial === 'function' ? initial() : initial })); return [state.value, value => { const next = typeof value === 'function' ? value(state.value) : value; if (!Object.is(next, state.value)) { state.value = next; schedule(); } }]; },
    useRef(initial) { return slot(() => ({ current: initial })); },
    useEffect(action, deps) { const state = slot(() => ({})); if (!state.deps || deps.some((value, i) => !Object.is(value, state.deps[i]))) { state.deps = deps; effects.push(() => { state.cleanup?.(); state.cleanup = action(); }); } },
  };
  return { react, get current() { return current; }, mount(action) { render = action; run(); }, async flush() { for (let i = 0; i < 12; i++) await new Promise(resolve => setImmediate(resolve)); }, unmount() { alive = false; for (const state of slots) state.cleanup?.(); } };
}
