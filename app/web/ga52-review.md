# GA-52 implementation review - 28 October 2026

Branch: GA-19-develop-initial-web-application-interface. Current approved recovery clarification supersedes the earlier phone-login verification flow. Prior work preserved; reviewed for the user-authorized checkpoint and target-branch push. Publication status is recorded in the excluded local progress note. All work is app/web only.

## Current flows and state boundaries

- Normal successful mock phone/NIC/email login -> existing development-only demo Home session. Production remains on Login with mock feedback; Home/Recent preview access remains disabled there.
- Forgot password -> required Email/Phone choice, existing sample NIC validation and simulated human check. No method is preselected; changing it clears stale request feedback and is disabled while pending.
- Email recovery -> existing Check your email, using fixed d[three bullets]@example.com. No email is sent, and no reset link exists. No invented email-link redemption flow.
- Phone recovery -> existing Verify code, using fixed +94 77 *** 0000. Button reads Verify code. Public demo code123456, five-minute demo deadline,45-second resend cooldown, mock pending/success/invalid/expired and reset-on-resend retained.
- Recovery code success -> #/reset-password, never Home or a session. Reset reuses registration's demo policy and exact confirmation through validateDemoPasswords. No trimming, normalization or truncation. Mock service receives only an outcome, not passwords.
- Reset success stays on the page, clears both passwords, and says exactly: Demo reset successful. No password was changed. Go to login is available. Invalid/error submissions stay; errors also clear passwords after mock completion.
- App owns a separate transient recovery state(idle/phone/verified/complete), independent of demoSession. It is not a security credential or real recovery grant. Only recovery callbacks advance it. Leaving recovery or refreshing discards it. Direct verify/reset without progression shows Start the recovery demo with working Start recovery and Login links. Back cannot restore cleared progression. Completed reset cannot be resubmitted in the same page.
- App updates recovery stage and route together to avoid briefly showing a missing-progression screen between legitimate steps. All async pages ignore late results after unmount.

Development origin: http://127.0.0.1:5173. Routes: /#/login(empty root also Login), /#/register, /#/forgot-password, /#/check-email, /#/verify-code, /#/reset-password, /#/home, /#/recent. Hash router retained; earlier pathname migration is still pending and not claimed complete.

## Interface and design checklist

Complete below means the agreed mock frontend increment, never real authentication.

| Interface | Status | Design basis / limits |
| --- | --- | --- |
| Phone login | Complete | Existing Figma613:2372 and shared controls; direct DEV Home success restored, no code step. |
| NIC/email login | Complete | Figma635:2405; behaviour unchanged, real verification requirements pending GA-54. |
| Registration | Complete | Figma639:2509 plus approved policy/spacing/demo/legal adaptations; no account created. |
| Forgot password | Complete as approved adaptation | Figma635:2583 NIC/human check retained. Required Email/Phone selector, synthetic destinations and method-specific Continue labels are provisional additions; no selector frame supplied. |
| Check your email | Complete | Figma635:2669 with truthful mock copy, synthetic email and mock resend; no delivery/active30-minute link claim. |
| Recovery verification | Complete as repurposed adaptation | Reuses Figma635:2490 code layout; user explicitly reassigned it from login to recovery. Wording/back link/destination changed, code input/deadlines preserved. |
| New-password/reset | Complete as approved provisional addition | No Figma frame supplied. Reuses AuthLayout, PasswordField, Button, StatusMessage, Inter/shared link styles and existing registration policy. |
| Missing-progression fallback | Complete as provisional supporting UI | Shared AuthLayout, focused heading and working recovery/login links; no fabricated destination or session. |

Approved source desktop frames are1440x820. Shared Inter, header label, demo copy, proportional photo and spacing are earlier user-approved changes. Narrow browser layouts, recovery selector and reset screen remain provisional website adaptations, not approved Figma designs. Original/photo assets untouched this increment; no new fonts/images/dependencies.

## Acceptance checklist

| Criterion | Status | Evidence / limitation |
| --- | --- | --- |
| 1. Interfaces match approved Figma designs | Incomplete for final design sign-off | Supplied frames reused; selector/reset/fallback have no supplied designs and are explicitly provisional. User approved implementing these adaptations, not a claim of exact Figma match. |
| 2. Required fields/input validation | Complete for approved mock rules | Existing fields plus required recovery choice/code, shared Unicode demo password policy and exact confirmation tested. Real identity/policy contracts are GA-54. |
| 3. Masking and visibility controls | Complete | Login/registration/reset masked by default, independent controls, submission clearing and pending concealment tested. |
| 4. Loading/success/error states | Complete for mocks | Login, registration, recovery, resend, code invalid/expired and reset error/success pass. No delivery or changed-password claims. |
| 5. Authentication-screen navigation | Complete with current hash router | Normal login->DEV Home; Email->confirmation; Phone->code->reset; return/restart/keyboard links, direct/refresh/Back guards pass. No recovery callback can start a session. Pathname migration remains separate. |
| 6. Responsive supported browser sizes | Incomplete beyond tested Edge matrix | Implemented pages have zero horizontal overflow/natural scrolling in checked desktop/narrow widths; code12 + reset10 states per build. No approved mobile frames, other engines/physical-device coverage not available. |
| 7. Relevant UI/interaction tests | Complete for implemented/tested Edge scope | Build/lint, validator/service and affected browser suites pass; wider browser/screen-reader acceptance remains unverified. |

GA-52 mock interface inventory is implemented. Full design/browser acceptance remains open. Real lookup, endpoints, email/SMS delivery, password change, verification/expiry enforcement, Keycloak and real sessions remain GA-54. The DEV session boolean is not full GA-53 protected routing. GA-21 chat is unchanged.

## Latest verification and review

- Build and lint pass without warnings; all existing login/registration/recovery/code unit suites pass. New check-password-reset.mjs verifies shared Unicode code-point/digit/symbol policy, exact whitespace/confirmation and outcome-only reset mock.
- check-login-browser:48 phone layout cases plus NIC/email validation/switching, required/error/pending/success, password clearing, direct normal DEV Home navigation and logout.
- check-recovery-browser: required unselected method/error focus, Email branch, pending inherited radio disablement, duplicate guard, synthetic confirmation/resend, history/refresh/unmount and28 responsive cases.
- check-verification-browser in development AND built production: Phone recovery->code->reset, typed/pasted digits, keyboard/arrows/Backspace, required/incomplete/ARIA, mock invalid/expired/success, elapsed5-minute deadline and45-second cooldown/resend, duplicate guards, reset policy/masking/exact password/error/success/clearing, no session, direct/refresh/Back fallback and keyboard recovery link.12 code +10 reset layout cases per build, actual Inter/images/zero horizontal overflow/reachable footer. Synthetic clock advances avoid long sleeps while exercising deadline logic.
- check-demo-navigation-browser development/production: phone/NIC/email normal success/error/invalid, DEV logout/Back/refresh, no production demo access, existing registration/recovery URLs preserved.
- Shared auth UI/account suite: actual fonts across six existing pages,30 responsive cases, registration action, menu focus/Escape/Tab/activation/logout and no session resurrection. Registration browser16 cases passes after shared password helper extraction.
- Unchanged shell/photo/link suites' latest results reused; shared photo CSS/assets and text-link styling were not changed here. Reviewed new selector/reset1440/375 screenshots, plus updated verification. Evidence remains ignored under node_modules/.tmp.
- Browser harness uses fresh profiles and disables extensions to avoid extension-driven autofill/requests affecting tests. Inherited fieldset disabled controls are checked with :disabled, retaining meaningful pending locks. No application Fetch/XHR, browser storage, cookies or runtime exceptions observed in tested flows.
- Reviewed routing/state, all changed UI/services/validation and tests. Separate recovery state and no session callbacks in recovery are explicit. No password is passed to reset service or stored; no credentials in URLs/logging. No new dependencies or out-of-frontend changes. Existing uncommitted work preserved.

## Changed-file guide

App.tsx owns routing, existing demo session and separate recovery progression; LoginPage restores direct demo success. ForgotPasswordPage/recovery.css add the selector and mock branch. VerificationCodePage preserves entry/deadlines but uses onVerified for reset, never onDemoSuccess. ResetPasswordPage owns reset fields/feedback, RecoveryRequiredPage owns safe fallback; both use AuthLayout. registrationValidation exports validateDemoPasswords for registration/reset reuse; mockPasswordResetService accepts only an outcome. Browser/unit scripts cover corrected flows. README and excluded local plan/progress updated.

## Checkpoint message

    GA-52: implement authentication and password-recovery interfaces

    Add shared auth layout, NIC/email login, registration and recovery UI.
    Separate normal DEV demo-login navigation from Email/Phone recovery.
    Reuse code verification for recovery and add a provisional reset preview;
    no delivery, password change, real verification or session is performed.

    Include supporting DEV-only Home/account logout, shared Inter/link styling,
    proportional photo framing and focused validation/navigation/browser tests.
    Keep real integration under GA-54, with provisional design/browser gaps noted.

Reviewed checkpoint files (relative to app/web):

- README.md
- figma-login-assets.md
- ga52-review.md
- public/assets/figma/login/auth-coast.jpg
- public/assets/figma/login/mail.svg
- public/assets/figma/login/mobile-tab-inactive.svg
- public/assets/figma/login/nic-email-tab-active.svg
- public/assets/figma/login/recovery-mail-check.svg
- public/assets/figma/login/register-info.svg
- public/assets/figma/login/register-nic.svg
- public/assets/figma/login/register-user.svg
- scripts/check-auth-links-browser.mjs
- scripts/check-auth-photo-browser.mjs
- scripts/check-auth-ui-browser.mjs
- scripts/check-demo-navigation-browser.mjs
- scripts/check-layout-browser.mjs
- scripts/check-login-browser.mjs
- scripts/check-login.mjs
- scripts/check-password-reset.mjs
- scripts/check-recovery-browser.mjs
- scripts/check-recovery.mjs
- scripts/check-registration-browser.mjs
- scripts/check-registration-spacing.mjs
- scripts/check-registration.mjs
- scripts/check-verification-browser.mjs
- scripts/check-verification.mjs
- src/App.tsx
- src/components/PasswordField.tsx
- src/components/auth/AuthLayout.tsx
- src/components/auth/LoginCredentials.tsx
- src/components/auth/LoginMethodSwitch.tsx
- src/components/auth/VerificationCodeInput.tsx
- src/components/layout/ApplicationHeader.tsx
- src/components/layout/ApplicationLayout.tsx
- src/components/layout/Sidebar.tsx
- src/index.css
- src/pages/ForgotPasswordPage.tsx
- src/pages/LayoutPreviewPage.tsx
- src/pages/LoginPage.tsx
- src/pages/RecoveryRequiredPage.tsx
- src/pages/RegistrationPage.tsx
- src/pages/ResetEmailConfirmationPage.tsx
- src/pages/ResetPasswordPage.tsx
- src/pages/VerificationCodePage.tsx
- src/services/mockCodeService.ts
- src/services/mockPasswordResetService.ts
- src/services/mockRecoveryService.ts
- src/services/mockRegistrationService.ts
- src/styles/application.css
- src/styles/components.css
- src/styles/login.css
- src/styles/recovery.css
- src/styles/registration.css
- src/styles/verification.css
- src/validation/codeValidation.ts
- src/validation/loginValidation.ts
- src/validation/recoveryValidation.ts
- src/validation/registrationValidation.ts

Excluded: AGENTS.md, SPRINT-1-TASKS.md, LOCAL-PROGRESS.md, secrets/env files, node_modules, dist and generated evidence/profiles. Local instruction/progress files and generated artifacts stay excluded.
