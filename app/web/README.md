# GovAI phone login preview

React, TypeScript, Vite, and plain CSS. Existing dependencies and Figma assets only.

## Local preview

Run `npm run dev` (or `npm.cmd run dev` in PowerShell where script execution is restricted).

This is a frontend demo, not authentication. Phone/password submissions never leave the browser. No session, cookie, token, credential storage, or authenticated redirect is created. The mock adapter receives only the selected demo outcome, waits 800 ms, and returns success or failure. Expand “Demo response controls” to select the result. Use sample details only. Password input is cleared after either result.

Phone validation accepts `07XXXXXXXX` or `+947XXXXXXXX`, allowing spaces and hyphens. Normalisation to +94 is a provisional UI rule, not verification of an active mobile number. Passwords are required and are not trimmed or checked against invented registration rules.

The approved frontend methods are phone/password and NIC-or-email/password; only phone login is implemented. Backend support is unconfirmed. NIC/email, remember-me, password reset, registration, other languages, and help are visibly unavailable. The human check is simulated, not CAPTCHA. The photo is static; other slides were not supplied.

## Source structure

- `src/main.tsx` mounts React and loads base styles from `src/index.css` and reusable-control styles from `src/styles/components.css`.
- `src/App.tsx` selects local hash previews: empty hash or `#/login` preserves LoginPage; `#/home` and `#/recent` demonstrate the shared website layout. Unknown hashes show an unavailable message. This is preview navigation, not authentication or protected routing.
- `src/pages/LoginPage.tsx` owns login input state, submission, and page layout.
- `src/components/` contains separate `Brand`, `Icon`, `Button`, `Checkbox`, `InputField`, `PasswordField`, and `StatusMessage` files. `PasswordField` builds on `InputField`.
- `src/validation/loginValidation.ts` contains phone normalisation and required-field rules.
- `src/services/mockLoginService.ts` simulates the selected result without receiving credentials.
- `src/styles/login.css` controls login layout, page typography, and responsive rules. `src/styles/components.css` controls the shared brand, icons, fields, checkboxes, buttons, spinner, and status messages. `src/index.css` contains global defaults and shared colour variables. Images and icons remain in `public/assets/figma/login/`.

Typing updates state in `LoginPage`. Submission validates those values, focuses the first invalid field, or calls the mock service with only the selected outcome. The returned result updates the status message and clears the password. This boundary can be replaced once the backend contract is agreed.

## Design limitations

Source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/?node-id=613-2372

The login preview retains its system sans-serif and Georgia fallbacks, which affect text metrics and wrapping. The new shell uses locally hosted official Inter, Montserrat and Fraunces binaries with bundled OFL licences; see `ga51-design-assets.md`. The desktop photo uses the original asset with the supplied crop and overlay. Below 900px the photo is hidden as a provisional responsive adaptation. Demo explanations and unavailable labels add height beyond the Figma card. The preview footer and caption avoid claiming official deployment or working multilingual AI.

## Checks

- `npm run build`: TypeScript and production bundle.
- `npm run lint`: Oxlint.
- `node scripts/check-login.mjs`: validation and mock adapter checks (Node 24, no additional dependencies).
- With the Vite server running at `http://127.0.0.1:5173`, `node scripts/check-login-browser.mjs`: headless interaction and responsive checks using installed Windows Edge. Requires permission to launch a browser; no test libraries are installed. Uses an automatically allocated local debugger port and writes its isolated profile, layout measurements, and desktop screenshot under `node_modules/.tmp/`. Debugger commands time out after 10 seconds.

Real registration, login, logout, session handling, and backend security verification remain pending under GA-53/GA-54. These checks do not establish backend integration or full accessibility conformance.

## GA-51 layout preview handover

Open `http://127.0.0.1:5173/#/home` or `/#/recent`; the unchanged base URL and `/#/login` show phone login. These URLs do not represent authenticated sessions.

- `src/components/layout/`: ApplicationLayout composes header, sidebar and content, and owns the native modal menu/refocus/Tab endpoints. ApplicationHeader owns the brand/account row. Sidebar composes reusable NavigationItem links and disabled destinations. ContentContainer provides a semantic, focusable main region.
- `src/pages/LayoutPreviewPage.tsx`: synthetic/static preview copy, disabled composer, suggestion samples and unavailable history panel. Hash changes update App selection and Sidebar aria-current; no server calls, conversations or user session are created.
- `src/styles/application.css`: shell layout, colors, type, spacing and responsive rules. Font faces are shell aliases, preserving login typography. Assets: `public/assets/figma/shell/`; fonts/licences: `public/assets/fonts/`; provenance and deliberate differences: `ga51-design-assets.md`.
- Brand supports an optional subtitle/emblem; Icon supports a shell collection using natural export dimensions. Defaults preserve login usage. No new dependency installed.
- At <=900px the proposed website adaptation replaces the desktop sidebar with a labelled modal menu. Tab/Shift+Tab cycle between controls; Escape, Close and destination selection return focus to Menu; widening to desktop closes it and focuses main content. Mobile/tablet Figma frames were not supplied. The separate mobile app is untouched.
- With Vite at port 5173, run `node scripts/check-layout-browser.mjs` for 16 layout cases, desktop geometry/active colors, all shell asset dimensions and custom font glyph use, keyboard navigation, menu behaviour at four widths, history and empty storage. Artifacts: `node_modules/.tmp/ga51-*.png` and `ga51-layout-measurements.json`. Run the existing login browser script separately for its 48 cases and input/mock interactions. Both require headless Edge launch permission.
