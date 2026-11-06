const { mkdirSync } = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const projectRoot = path.resolve(__dirname, '..');
const temporaryDirectory = path.join(projectRoot, '.cache', 'tmp');
mkdirSync(temporaryDirectory, { recursive: true });

// Keep Expo settings, Metro caches and compiler temporary files in this app.
const result = spawnSync(
  process.execPath,
  [
    path.join(path.dirname(require.resolve('expo/package.json')), 'bin', 'cli'),
    ...process.argv.slice(2),
  ],
  {
    cwd: projectRoot,
    stdio: 'inherit',
    env: {
      ...process.env,
      __UNSAFE_EXPO_HOME_DIRECTORY: path.join(projectRoot, '.cache', 'expo-home'),
      EXPO_NO_TELEMETRY: '1',
      DOTSLASH_CACHE: path.join(projectRoot, '.cache', 'dotslash'),
      TMP: temporaryDirectory,
      TEMP: temporaryDirectory,
      TMPDIR: temporaryDirectory,
    },
  },
);

if (result.error) {
  console.error(result.error.message);
}
process.exit(result.status ?? 1);
