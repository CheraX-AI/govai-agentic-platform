const path = require('node:path');
const { spawnSync } = require('node:child_process');

// Explicit development command; never writes an env file or starts a native build.
console.log('Development preview: Account Created — sample content only. No account is created.');
const result = spawnSync(
  process.execPath,
  [path.join(__dirname, 'expo.cjs'), 'start', '--go', '--lan'],
  {
    cwd: path.resolve(__dirname, '..'),
    stdio: 'inherit',
    env: { ...process.env, EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW: '1' },
  },
);
if (result.error) console.error(result.error.message);
process.exit(result.status ?? 1);
