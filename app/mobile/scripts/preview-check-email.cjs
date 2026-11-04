const path = require('node:path');
const { spawnSync } = require('node:child_process');
const args = process.argv.slice(2);
if (args.length && !(args.length === 2 && args[0] === '--port' && /^\d+$/.test(args[1]) && Number(args[1]) >= 1 && Number(args[1]) <= 65535)) {
  throw Error('Optional supported argument: --port NUMBER');
}
console.log('Development preview: Check Email — static sample only. No email has been sent.');
const result = spawnSync(process.execPath, [path.join(__dirname, 'expo.cjs'), 'start', '--go', '--lan', ...args], {
  cwd: path.resolve(__dirname, '..'), stdio: 'inherit',
  env: { ...process.env, EXPO_PUBLIC_CHECK_EMAIL_PREVIEW: '1', EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW: '0' },
});
if (result.error) console.error(result.error.message);
process.exit(result.status ?? 1);
