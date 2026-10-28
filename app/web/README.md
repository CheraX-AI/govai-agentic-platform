# GovAI website authentication previews

React, TypeScript, Vite, and plain CSS. Existing dependencies and Figma assets only.

## Local preview

Run `npm run dev` (or `npm.cmd run dev` in PowerShell where script execution is restricted).

This is a frontend demo, not authentication. Phone/password submissions never leave the browser. No real session, cookie, token, credential storage, or authenticated redirect is created. Development-only demo navigation uses an in-memory flag, as described below. The mock adapter receives only the selected demo outcome, waits 800 ms, and returns success or failure. Expand “Demo response controls” to select the result. Use sample details only. Password input is cleared after either result.

Phone validation accepts `07XXXXXXXX` or `+947XXXXXXXX`, allowing spaces and hyphens. Normalisation to +94 is a provisional UI rule, not verification of an active mobile number. Passwords are required and are not trimmed or checked against invented registration rules.

Both phone/password and NIC-or-email/password are implemented as mock previews, selected through the login method buttons. Backend support is unconfirmed. Registration is also available as a local mock preview. Remember-me, real password reset, other languages, and help remain unavailable; mock recovery/reset UI is available. The human check is simulated, not CAPTCHA. The photo is static; other slides were not supplied.

## Source structure

- `src/main.tsx` mounts React and loads base styles from `src/index.css` and reusable-control styles from `src/styles/components.css`.
- `src/App.tsx` selects local hash previews: empty hash or `#/login` preserves LoginPage; `#/home` and `#/recent` demonstrate the shared website layout. Unknown hashes show an unavailable message. This is preview navigation, not authentication or protected routing.
- `src/pages/LoginPage.tsx` owns login input state, validation, switching, submission and form contents. `src/components/auth/AuthLayout.tsx` renders the shared header, banner, card/form wrapper, footer and photo panel; it loads the existing login CSS.
- `src/components/` contains separate `Brand`, `Icon`, `Button`, `Checkbox`, `InputField`, `PasswordField`, and `StatusMessage` files. `PasswordField` builds on `InputField`.
- `src/validation/loginValidation.ts` contains phone normalisation and required-field rules.
- `src/services/mockLoginService.ts` simulates the selected result without receiving credentials.
- `src/styles/login.css` controls login layout, page typography, and responsive rules. `src/styles/components.css` controls the shared brand, icons, fields, checkboxes, buttons, spinner, and status messages. `src/index.css` contains global defaults and shared colour variables. Images and icons remain in `public/assets/figma/login/`.

Typing updates state in `LoginPage`. Submission validates those values, focuses the first invalid field, or calls the mock service with only the selected outcome. The returned result updates the status message and clears the password. This boundary can be replaced once the backend contract is agreed.

## Design limitations

Source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/?node-id=613-2372

Authentication pages and Home/Recent previews now use the existing locally hosted Inter font, including headings and controls, per user approval. Montserrat preserves the GovAI wordmark. Shared font faces live in src/index.css; the original font assets and OFL licences remain in public/assets/fonts (see ga51-design-assets.md). The desktop photo uses the derived coastal crop described below. Below 900px the photo is hidden as a provisional responsive adaptation. Demo explanations and unavailable labels add height beyond the Figma card. The preview footer and caption avoid claiming official deployment or working multilingual AI.

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
- `src/styles/application.css`: shell layout, colors, type, spacing and responsive rules. Shared font faces are defined in src/index.css; both auth and shell use Inter, preserving the Montserrat wordmark. Assets: `public/assets/figma/shell/`; fonts/licences: `public/assets/fonts/`; provenance and deliberate differences: `ga51-design-assets.md`.
- Brand supports an optional subtitle/emblem; Icon supports a shell collection using natural export dimensions. Defaults preserve login usage. No new dependency installed.
- At <=900px the proposed website adaptation replaces the desktop sidebar with a labelled modal menu. Tab/Shift+Tab cycle between controls; Escape, Close and destination selection return focus to Menu; widening to desktop closes it and focuses main content. Mobile/tablet Figma frames were not supplied. The separate mobile app is untouched.
- With Vite at port 5173, run `node scripts/check-layout-browser.mjs` for 16 layout cases, desktop geometry/active colors, all shell asset dimensions and custom font glyph use, keyboard navigation, menu behaviour at four widths, history and empty storage. Artifacts: `node_modules/.tmp/ga51-*.png` and `ga51-layout-measurements.json`. Run the existing login browser script separately for its 48 cases and input/mock interactions. Both require headless Edge launch permission.

## GA-52 NIC/email login increment

At the existing login URL, select NIC or email. Provisional format validation accepts nine digits plus V/X (case-insensitive), twelve digits, or basic email syntax with no internal whitespace. Surrounding identifier whitespace is trimmed on blur/submission; passwords are only required and are never trimmed. This checks format, not identity or email ownership.

`LoginPage.tsx` still owns inputs, touched errors, submission and outcomes. `components/auth/LoginMethodSwitch.tsx` renders labelled pressed-state buttons; `LoginCredentials.tsx` composes the existing InputField and PasswordField for either method. Validation is in `validation/loginValidation.ts`. Switching methods clears identifier/password/human-check/errors/results and resets password visibility; it is disabled while submitting. The shared mock service continues to receive only the selected result, not credentials. No authentication, sessions, network integration or additional pages were added.

`styles/login.css` adapts the existing segmented styling from spans to buttons. The obsolete Coming later label is removed, so the card is naturally shorter; page panels, photo/crop, typography, other controls and responsive rules remain. Three exact new Figma icons are documented in `figma-login-assets.md`. Extended `check-login.mjs` and `check-login-browser.mjs` cover both methods, reset-on-switch/pending/keyboard and responsive checks.

AuthLayout is a presentational shell: pass the heading ID, native form props, form contents as children and optional footer controls. LoginPage owns the state behind those controls. Change shared authentication markup in AuthLayout; keep page-specific input/mock logic in the page. The refactor preserves existing DOM classes/styles and both login flows; no registration interface is included.

## GA-52 registration demo

Open `http://127.0.0.1:5173/#/register`, or use Create an account (demo) on login. `RegistrationPage.tsx` owns values, touched errors, mock submission and demo response controls. `AuthLayout` accepts optional `wide` (default false) for the 520px registration card; login stays 440px. `PasswordField` accepts optional label (default Password); confirmation visibility is independent. `styles/registration.css` scopes wider-card/paired-field rules and stacks rows at <=580px as a proposed responsive website adaptation, not an approved mobile design.

All displayed registration fields and demo consent are required. Unicode names have no restrictive pattern. NIC, phone and email reuse the existing format helpers. Passwords require at least eight Unicode code points, one ASCII digit 0-9, and one Unicode punctuation/symbol character (Unicode P or S category); whitespace is not a symbol. Passwords are never trimmed, normalized or truncated; confirmation must match exactly. This is a demo policy and MUST be aligned with the supplied real authentication system in GA-54. Format checks do not verify identity, phone service or email ownership.

Terms starts unchecked and is explicitly demo-only; unavailable Terms/Privacy are plain text, with no fabricated legal content or dead links. Success stays on registration and says "Demo registration successful. No account was created." with a prominent Go to login action; both passwords are cleared after success or failure. Back to login is always available. The mock adapter receives only the selected outcome: no registration data, SMS/email delivery, account/session, token or chat redirect. Late mock results after leaving are ignored. No sensitive data is stored in browser storage or URLs.

Changes: `validation/registrationValidation.ts`, shared format helpers in `loginValidation.ts`, `services/mockRegistrationService.ts`, `pages/RegistrationPage.tsx`, `styles/registration.css`; App adds public #/register selection/title. Three original Figma icons are recorded in figma-login-assets.md. `node scripts/check-registration.mjs` verifies Unicode policy boundaries, required inputs and mock outcomes; with Vite at port 5173, `node scripts/check-registration-browser.mjs` checks interaction/navigation and 16 initial/error layout cases. It uses the existing headless Windows Edge pattern and writes registration screenshots/measurements only under node_modules/.tmp.

Registration spacing checks: `node scripts/check-registration-spacing.mjs [output-json] [viewport-width] [viewport-height]` measures initial, validation-error, expanded demo information and combined states independently at 100% scale. Standard eight sizes are always covered. The concise warning, password hint and unavailable legal note remain visible; extended demo/policy explanations are in the labelled footer disclosure. Registration-only padding/gaps are scoped through AuthLayout's wide class; login defaults are unchanged. Expanded information and error states may legitimately require natural scrolling. Measurement JSON and screenshots remain under node_modules/.tmp.

Shared authentication photo: `styles/login.css` controls `.photo-panel` and `.login-photo`. The panel is viewport-height/sticky while the form scrolls naturally. The original portrait photo uses a proportional bottom-centred cover crop with lower-scene framing; it no longer enlarges when errors/demo disclosures grow. Do not resize the form to compensate for its photo. `node scripts/check-auth-photo-browser.mjs after` verifies 56 login/registration state/viewport cases and records crop/overlay/form measurements under node_modules/.tmp. Run with `before` only when intentionally capturing a new baseline; keep generated artifacts uncommitted.

### Mock password recovery preview
- Open `http://127.0.0.1:5173/#/forgot-password` or follow Forgot password from either login method. `#/check-email` also works directly or after a successful mock request.
- ForgotPasswordPage owns NIC/human-check input and local validation; recoveryValidation reuses the provisional NIC format rule. Only the selected success/error outcome reaches mockRecoveryService, never the NIC. ResetEmailConfirmationPage owns resend feedback and a fixed masked synthetic address independent of input. No lookup, email, reset link, session or expiry enforcement exists. No resend countdown; duplicate pending actions are guarded.
- Both pages compose AuthLayout and shared controls. Change recovery copy/structure in their page files, recovery-only colours/spacing in styles/recovery.css, shared header/photo/card/footer in components/auth/AuthLayout.tsx and styles/login.css. Existing login/registration defaults are preserved.
- The 30-minute lifetime is explicitly a design preview. Real recovery is GA-54; code verification now belongs to Phone password recovery, not login. Desktop references: Figma 635:2583 and 635:2669. Narrow layouts reuse the proposed responsive website adaptation, not an approved mobile design.
- Checks: node scripts/check-recovery.mjs; node scripts/check-recovery-browser.mjs (local Vite on port5173 and Edge required). Browser evidence/screenshots live under ignored node_modules/.tmp.

### Shared authentication coastal photo
AuthLayout uses public/assets/figma/login/auth-coast.jpg, cropped from the preserved original login-photo.jpg at x0/y620/w985/h977. No resizing or generated image content. styles/login.css uses a panel-sized image with cover/top-right anchoring, retaining the flag and lighthouse top across checked desktop ratios. Narrow panels sacrifice left sea; wide panels sacrifice lower rocks. Change this shared asset/selector for all four auth screens; do not resize forms to compensate for the photograph.

### Development-only demo login navigation
Successful phone/NIC/email mock login opens `http://127.0.0.1:5173/#/home`. App.tsx owns a boolean demo-session flag in React memory only; LoginPage receives a development-only success callback. Invalid and failed submissions stay on Login. Home/Recent show a Demo session - not authenticated notice and Demo logout while that flag is set. Logout returns to `#/login` and clears it; refresh clears it too. Browser history never restores a logged-out flag. Direct development Home/Recent URLs remain static labelled layout previews without a session; they do not establish authentication.

Production builds do not run the success-navigation callback or render Home/Recent previews; these routes show Development preview unavailable. Public login/registration/recovery mock UI remains explicitly mocked. This is not Keycloak state, a real session, route authorization, or GA-54 integration. No API calls, credentials or session flag are stored. Late login responses after leaving the page are ignored.

Current hash routes: `#/login`, `#/register`, `#/forgot-password` -> `#/check-email`, `#/home`, `#/recent`. Pathname-router migration is still pending; normal mock login goes directly to Home in development; Phone recovery enters #/verify-code then #/reset-password. Checks: `node scripts/check-demo-navigation-browser.mjs` against dev port5173; add `production` against built Vite preview port4173.

### Shared typography, link states and account disclosure
Authentication headers show a noninteractive English label. User-facing demo notices use a normal hyphen. Shared text links in styles/login.css use brand normal/visited colour, a darker hover colour,0.2em underline offset,0.08em thickness and skip-ink:auto; the global3px keyboard focus outline remains. Registration primary Go to login keeps white text on brand background.

ApplicationHeader owns the Harshan account disclosure and focus/Escape/outside-click/Tab dismissal. ApplicationLayout passes through the existing App logout callback from LayoutPreviewPage; no second session mechanism exists. Active demo sessions can log out through either the notice shortcut or account action. Static previews clearly report no session and disable logout.

See [GA-52 review](ga52-review.md) for the current interface/acceptance audit, evidence, proposed58-file checkpoint and commit message. Verification UI is implemented; pathname routing remains outstanding; real-system integration is GA-54.

### Password recovery demo - current flow

Normal mock phone/NIC/email login opens Home only in development, without a code step. Forgot password requires Email or Phone selection plus sample NIC/human check. Email opens Check your email; Phone opens Verify code. Demo code123456 succeeds; five-minute demo expiry and45-second resend cooldown remain. Verify code leads to the provisional #/reset-password screen, never a session or Home. No email/SMS is sent.

Reset uses the existing registration demo password policy and exact confirmation; its service receives only the selected outcome. Success says Demo reset successful. No password was changed., clears both passwords and offers Go to login. Recovery state is separate transient App state; leaving or refreshing clears it. Direct verify/reset without progression offers working Start recovery and Login links. No recovery grant/token is stored or security enforcement claimed.

Selector, reset and fallback are user-approved provisional UI additions because no Figma frames were supplied for them. Reused supplied verification frame is adapted for recovery wording/navigation. Shared photo/fonts/link styles remain unchanged.

Change responsibilities: App.tsx routes/state; ForgotPasswordPage + recovery.css selector; VerificationCodeInput/codeValidation/mockCodeService code input and simulated deadlines; VerificationCodePage recovery feedback; ResetPasswordPage and mockPasswordResetService reset simulation; registrationValidation.validateDemoPasswords shared rules; RecoveryRequiredPage safe direct/refresh fallback. Run check-password-reset.mjs and check-verification-browser.mjs (add production for built preview port4173), along with login/recovery/navigation regressions.
