# GovAI mobile

Expo + React Native + TypeScript implementation of the completed Figma loading
page (`47:3`) and welcome page (`278:709`). The original loading component and
its artwork/styles are preserved. Startup loads all eight bundled images and
four Inter weights, then immediately replaces loading with welcome. There is
no timer, transition animation, navigation stack or persisted onboarding state.
Welcome appears after startup on every launch; Android Back cannot return to
loading. Fast local loading may make the loading screen very brief.

## Unfinished interactions

Get started, Log in and the EN language selector are deliberately disabled
Pressables with no action handlers and accessibilityState.disabled=true.
Their Figma appearance is retained; TalkBack labels identify them as unavailable.
No hidden onboarding sheet, login, translations, backend calls or GA-22 chat
are implemented. No additional dependencies were installed.

## Welcome validation — October 19, 2026

- TypeScript check passed.
- Android production export passed, including Hermes compilation, eight images
  and four bundled fonts. The first sandbox attempt could not execute Hermes;
  the export succeeded with execution permission.
- Original hero and all three rasterized icons inspected; asset slots and
  cover geometry reviewed against the Figma screenshot. No Android screenshot
  comparison or physical-device testing has been performed for welcome.
- AGENTS.md remains unchanged. No commits, pushes or branch switches performed.

### Test welcome on your Android phone

Run `npm.cmd start` in this directory with phone and laptop on the same Wi-Fi,
then scan its QR code in your compatible Expo Go app. Reload or reopen the app:

1. Confirm loading retains its approved appearance if visible, then welcome
   appears without a deliberate pause or fade. Reopen to repeat startup.
2. Compare the hero/logo crop, lower dark overlay, heading, burgundy button,
   gold Log in text and EN selector with Figma frame 278:709.
3. Tap Get started, Log in and EN: each must do nothing and open no screen.
4. Press Android Back: it may leave the app/return to Expo Go, but must never
   show loading as a previous route. Reopen to confirm welcome after startup.
5. Check status/navigation bars and camera cutout do not obscure controls.
   Repeat with gesture and three-button navigation if available.
6. Enable larger system text and TalkBack. Confirm content can scroll when
   needed, text stays readable and controls announce their disabled state.

If startup resources fail, a visible error asks you to reload instead of
silently showing an incomplete welcome screen. Report any layout mismatch
with your phone model and text-size setting.

## Open on an Android phone (Windows PowerShell)

Install/update Expo Go compatible with **SDK 57** on your phone:
https://expo.dev/go?device=true&platform=android&sdkVersion=57

Expo's download page listed SDK 57 as the current Expo Go release when checked
on October 16, 2026. This project deliberately uses SDK 57 rather than npm's
newer SDK 58. A mismatching Expo Go version must be replaced with the SDK 57
Android build from that page. The installed version on your phone has not been
inspected.

Connect the computer and phone to the same Wi-Fi network. In PowerShell:

```powershell
Set-Location 'C:\Users\ASUS\Desktop\Gov-Ai\govai-agentic-platform\app\mobile'
npm.cmd start
```

Dependencies are already installed. On a fresh checkout, first run `npm.cmd ci`
from this directory. Use `.cmd` to avoid the machine's PowerShell script policy
restriction; there is no need to change execution policy.

Allow Node.js through Windows Firewall on your private network if prompted.
Open Expo Go, select **Scan QR code**, and scan the terminal QR code. Leave the
terminal running; Ctrl+C stops the server. If the default port is occupied:

```powershell
npm.cmd start -- --port 8082
```

If scanning fails, confirm the devices use the same network without client
isolation or a VPN blocking LAN traffic. You can also enter the terminal's
`exp://` address in Expo Go manually.

## Checks

```powershell
npm.cmd run typecheck
npm.cmd run check:expo
npx.cmd --yes expo-doctor
npm.cmd run export:android
```

Completed during implementation:

- TypeScript: passed.
- Expo dependency compatibility: dependencies up to date.
- Expo Doctor: 21/21 checks passed.
- Android production export: passed, including Hermes bytecode compilation.
  Four original Figma PNGs and two bundled Inter fonts were included.
- Expo Go LAN development server: started successfully; Android manifest
  returned HTTP 200, name GovAI, SDK 57.0.0, runtime exposdk:57.0.0 and a
  development bundle URL. The verification server was then stopped.
- Original asset files, fill geometry and screen layer order inspected against
  the Figma design. No physical-device screenshot comparison was performed.
- AGENTS.md SHA-256 stayed unchanged:
  BAB1B541B130FD7D597DFB2A6AB1E086F25DAE1F39B13FCAFBAE1049758D18B6.

Physical Android testing is still required. Check that all artwork appears,
the logo keeps its proportions, native status/navigation bars avoid content,
the screen stays visible, and Android Back uses the normal Expo Go behavior.
Also check larger font settings and TalkBack's loading announcement.

## Implementation details

Figma artwork is local under `assets/figma`; see its README for provenance and
the exact crop geometry. Inter fonts load locally through expo-font. System
chrome comes from the real device instead of Figma's iPhone mockup. Artwork
scales uniformly within safe-area insets, with bottom decoration anchored to
the bottom on taller phones. The static loading track follows Figma's layer
order, where the opaque logo crop covers the track.

The npm scripts use `scripts/expo.cjs` to keep Expo settings, temporary compiler
files, Metro caches and DevTools downloads in app-local ignored directories.
`.npmrc` also keeps npm's dependency cache here. No global packages are needed.
`dist/android` is a validation export, not an installable APK.

## Meet GovAI (GA-20) — October 25, 2026

Welcome Get started opens only Onboarding 0 - Meet GovAI (Figma frame 686:4783).
On-screen Back and Android Back return to Welcome, never loading. The Android
Back listener is removed when leaving Meet GovAI. Next and Skip remain disabled,
with disabled accessibility state and unavailable labels. Welcome language and
Log in remain disabled. The panel is not draggable; handle/dots are decorative.
No other onboarding pages, chat, account flow, or persistence is implemented.

The screen reuses Screen with zero horizontal outer padding, retaining all four
safe-area insets and its 480dp width limit. This optional padding override leaves
existing screens unchanged. The original illustration uses contain. Native text
wraps and follows system font scaling; the panel has a minimum height and grows,
and the whole screen scrolls naturally. Back/Next are 52/56dp; Skip has a minimum
48 x 48dp target. Existing loading/welcome artwork and wording are unchanged.
Startup preloads the three new assets alongside the original eight images and
four fonts, then replaces loading immediately without timers or added routes.

Validation: TypeScript passed. Original hero and rasterized arrow assets visually
inspected. These are asset/code checks, not rendered Android layout validation.
Previous physical-device responsiveness checks remain pending. No emulator or
physical-device navigation, TalkBack, or screenshot comparisons performed.

Phone checklist:
1. Open with compatible Expo Go; loading should immediately become Welcome when ready.
2. Tap Get started; compare Meet GovAI with frame 686:4783.
3. Confirm both Back actions return to Welcome; repeat several times.
4. Confirm Skip/Next and language/login do nothing; TalkBack announces disabled state.
5. Try narrow/short, tall, and wide windows where available; verify scrolling,
   subtitle wrapping, image proportions, cutouts, and both Android navigation modes.
6. Test default, enlarged, and maximum system fonts; all native text/actions must
   remain reachable. Handle and dots must not act as controls.

Android production export passed, including Hermes bytecode compilation, eleven
bundled images and four fonts. Export ran with execution permission for Hermes.
The export is not an installable APK and does not validate on-device rendering.

## Discover — October 25, 2026 (GA-20)

Meet GovAI Next opens Discover. Discover Back and Android Back return to Meet;
Meet Back still returns to Welcome. Discover Next and Skip remain disabled with
accessibility disabled states. Tiles are informational Views with no tap actions
or button semantics. Discover highlights the second dot by approval, Meet the
first. Panel is static; handle/dots are decorative. No other onboarding or chat.

Screen/layout helpers are reused unchanged. The hero uses Figma's verified crop
and explicit dimensions (see assets README). Native text scales and wraps; tiles
wrap using a font-aware basis and grow vertically. Content is bounded to 480dp,
safe areas handled once, natural scrolling preserved. Targets: Back 52dp, Next
56dp, Skip minimum 48 x 48dp. Existing Welcome and loading visuals are unchanged.

Validation: TypeScript passed; original hero and four rasterized icons visually
inspected; crop proportions and navigation source checked. This does not verify
native rendered geometry or TalkBack. No emulator or physical-phone tests were
performed. Previous physical-device checks, including Welcome enlarged-text
contrast, remain pending.

Phone checklist: walk Welcome -> Meet -> Discover and test both Back actions
repeatedly. Check disabled Next/Skip announcements, informational tile reading
order, second active dot, hero crop/logo, and natural scrolling at default, large,
and maximum text settings. Test narrow/short, tall and wide windows, camera
cutouts, and gesture/three-button navigation.

Android production export passed, including Hermes compilation. The exported
asset list contains 19 unique images/fonts (identical hero artwork deduplicated
by Metro). Source checks passed for Next wiring, Back mapping and disabled
Discover controls. Calculated tile columns at widths 320/360/400/600 and font
scales 1/1.3/2 confirm the intended wrap thresholds; these are mathematical
checks, not native layout or device tests.

## Ask onboarding (GA-20) — October 25, 2026

Discover Next opens Ask. Ask Back and Android Back return to Discover. Third
indicator highlighted by approval; Figma's second indicator is intentionally
overridden. Next/Skip disabled with accessibility disabled states. Static example
messages and informational English/Sinhala/Tamil chips have no actions, chat
requests, text input, backend integration, or language-changing behavior. The
example response is approved design copy, not dynamically generated guidance.

Artwork/arrow reuse verified by SHA-256 against Ask frame asset downloads. Text
is native and scalable; bubbles/chips wrap and grow; artwork stays proportional,
full-width and logo-preserving, with measured compact sizing and scroll fallback.
All controls retain safe areas and >=48dp targets. Shared helpers, Welcome and
Meet visuals remain unchanged; Discover changes only to enable Next. Global
status-bar settings unchanged. Official font sources/licenses are documented in
assets/figma/onboarding-ask/README.md; both licenses retained beside the fonts.

Validation: TypeScript passed; exact asset hashes, regular font metadata, Sinhala
and Tamil chip character cmap coverage, navigation source, disabled states and
absence of chat requests checked. These are source/font checks, not emulator or
phone tests. Previous physical-device checks remain pending, including Welcome
large-text contrast. Verify Sinhala/Tamil shaping, TalkBack reading order and
informational chips, Next/Skip disabled announcements, full logo, no hero gutters,
footer visibility and scrolling at default/large/max text sizes. Walk Welcome ->
Meet -> Discover -> Ask; test both Back paths repeatedly on Android. Test narrow,
short, tall and wide windows and both system navigation modes.

Final Android production export passed after correcting the Sinhala subset,
including Hermes bytecode and 21 bundled image/font assets. Both new fonts are
included. Export success does not establish native shaping or visual/device QA.

## Track onboarding (GA-20) — October 25, 2026

Ask Next opens Track; Track Back and Android Back return to Ask. Track Skip and
icon-only Next remain disabled with accessibility disabled states. Fourth dot
highlighted by approval. The example card, badge, stages and chevron have no tap
actions or button semantics, backend requests, real tracking or persistence.
Stage order: Application Submitted (complete), Documents Verified (complete),
Under Review (current), Approval (pending), Completed (pending).

Track geometry/crop were independently inspected against frame 284:799. Artwork
is explicit, proportional and edge-to-edge with complete-logo protection. Spare
height goes before the footer in the white panel, not into an expanding hero.
Natural content measurement excludes flexible space. Native text scales/wraps;
summary and stages reflow at narrow widths/large fonts. Controls retain >=48dp
targets and safe-area protection. Scrolling remains available if needed. Ask
changes only to enable Next; existing completed screen layouts/helpers remain.

Validation: TypeScript passed; hero/arrows matched by SHA-256; all seven card
raster assets inspected. Navigation source and stage order/status checked. These
checks do not establish rendered phone geometry or TalkBack behavior. Previous
physical-device checks remain pending; no emulator or phone tests performed.

Phone checklist: walk through all onboarding screens; test Track's on-screen
and Android Back repeatedly; check disabled Skip/Next announcements, fourth dot,
static card reading order/statuses, complete logo and no gutters, normal-text
fit, footer clearance, large/max text reflow and scrolling. Compare against
frame 284:799 at reference width; test short/tall/wide windows, cutouts and both
Android navigation modes. Prior Welcome contrast and script-shaping checks
remain pending.

Android production export passed, including Hermes compilation and 27 unique
bundled image/font assets after Metro deduplication. No dependencies installed.
Export and mathematical checks do not establish native visual/device behavior.

## Pay onboarding and five indicators — October 25, 2026

Track Next opens Pay. Pay Back and Android Back return to Track. Skip and Pay's
icon-only Next remain accessibly disabled. Cards are informational and preserve
exact sample wording: Payment confirmed / LKR 999.00 for GovAI Plus was paid. /
Just now; Application update / Your NIC renewal is now under review. / 10 minutes
ago. No payment/subscription behavior, payment data, backend integration, or
notification permissions are implemented.

OnboardingProgress now supplies five decorative, non-interactive indicators:
Meet 1, Discover 2, Ask 3, Track 4, Pay 5. Width is 80dp with original 8dp dots,
24dp active pill, 6dp gaps and existing colors. Existing screen changes were
compared against before copies: indicator replacement only, plus Track Next.
Welcome, shared Screen, layout constants and AGENTS.md remain unchanged.

Pay geometry and assets were verified independently against frame 284:908.
Its full-width artwork has explicit proportional dimensions and logo protection;
safe-area controls and bounded scalable native content remain separate. Flexible
space stays inside the white panel before the footer. Natural body/footer sizes
are measured separately to avoid feedback loops. Overflow scrolling remains.

Validation: TypeScript passed. Hero/arrows matched by SHA-256; all four new PNG
assets visually inspected. Source checks passed for sample wording, navigation,
disabled controls and absence of functional payment code. These do not establish
native layout or device accessibility. No emulator or physical-phone testing was
performed. Previous device checks, including Welcome large-text contrast and
Sinhala/Tamil shaping, remain pending.

Phone checklist: walk all five screens forward/back; check indicator positions,
Pay's disabled Skip/Next with TalkBack, informational card reading order and sample
copy, complete logo/no gutters, footer fit at default text, and wrapping/scrolling
at larger text. Compare Pay with Figma at reference width. Check short/tall/wide
windows, safe-area/cutout clearance, and gesture/three-button navigation.

Android production export passed with Hermes compilation and 31 unique bundled
image/font assets. Calculated five-marker footer width and 36 hypothetical
logo/height cases passed; these are mathematical checks, not phone tests.

## UI-only Login — October 25, 2026

Welcome Log in, all onboarding Skip controls and Pay Next open Login. Each entry
records its originating screen; visible Back and Android Back return there.
Existing onboarding order/indicators/layouts remain. Login unmounts on exit,
discarding transient React input state; the explicit Back clears inputs/state
and cleanup clears native input refs and dismisses the keyboard. Credentials are
not persisted, logged, sent to a server or placed in navigation state.

Sign-in is explicitly not connected. Log In validates required identifier and
password only, focuses the first invalid field, and exposes inline live errors.
Nonempty input only produces a no-request-sent notice, never authentication,
tokens, sessions, success routing or an invalid-credentials claim. No unconfirmed
National ID/email format rules. Password starts hidden and has an accessible
visibility toggle. Digital ID, Forgot Password and Sign up remain disabled with
accessibility disabled states. No social auth/dependencies/backend changes.

Read-only backend review found health routes only, no authentication endpoints.
The backend entry point also uses require despite package type=module; noted,
not changed or executed. Real sign-in remains unavailable in this repository.

Login reuses Screen without changing it, bounding its child to the viewport and
using a local ScrollView. Four safe edges are padded once. Focus scrolls fields
into view. Keyboard overlap is measured against the actual root rectangle:
resized Android windows add zero overlap; overlays add only the remaining overlap
as scroll-bottom space, avoiding blanket double keyboard compensation. This
coordinate handling needs phone validation. Large text wraps; form widths are
bounded; decoration yields before text. Eye/Back/links/buttons retain >=48dp
areas. Original Figma artwork and icon provenance are recorded in assets README.

Checks: TypeScript passed. Actual Login component logic executed with mocked
native controls: required errors/focus, secure toggle, valid-but-disconnected
submit, disabled links, explicit exit clearing and cleanup passed. These are
component-logic checks, not native/device tests. Artwork hashes match originals;
all four native icons visually inspected. Backend/shared helpers/instructions
verified unchanged. No actual credentials were used in checks.

Phone checklist: open Login from Welcome and every onboarding Skip plus Pay
Next; test both Back paths and re-entry with blank fields. Test blank/partial
submission, no National ID format rejection, password reveal and disabled links
with TalkBack. With Android keyboard open, reach both fields, errors, visibility
control and submit; test Next/Done, drag dismissal, gesture/three-button modes,
cutouts, short/wide windows and largest fonts. Verify no success screen/network
request. Android Back may dismiss the keyboard before normal navigation. Compare
artwork/form with Figma 54:14. Previous physical-device checks remain pending;
no emulator or phone keyboard/TalkBack/screenshot testing was performed.

Android production export passed including Hermes compilation and 35 unique
bundled image/font assets. Source checks verified all six Login origins, both
Back mappings, all Skip actions and Pay Next. Keyboard overlap arithmetic tested
for resized/overlay windows; actual native keyboard behavior remains untested.


## English-only restoration - October 26, 2026

The app uses direct English Text/TextInput content again. Welcome retains its disabled EN pill; there is no language menu, language-testing route, locale state, persistence or storage warning. Original onboarding routes, responsive styles and Login keyboard/form handlers are preserved. Ask retains its informational English/Sinhala/Tamil chips and the two fonts/licenses they require. Expo remains 57.0.31.

Removed the translation provider/dictionaries/font helpers, localized text wrapper, menu component, localization-only test script and eight unused static font weights. AsyncStorage and its exclusive merge-options/is-plain-obj dependencies were uninstalled. No active source references remain to the removed feature. Historical ignored cache snapshots/logs may retain old references; they are not runtime code.

Saved-data inspection: the removed provider used only govai.language.v1 with en/si/ta values. No connected device or its actual saved value was accessible. A previously stored value is inert and does not need deletion for English-only behavior. No application storage was cleared. If device-level cleanup is requested later, remove only this key; never clear all storage or uninstall the app merely to remove this preference.

Validation: TypeScript passed; Expo compatibility reports dependencies up to date; Android/Hermes export passed (35 assets). Mocked checks executed actual Startup navigation (Welcome, all five onboarding screens, all Login origins and Back paths) and Login validation/focus, password visibility, disconnected submit, disabled actions, explicit exit and cleanup. Source comparisons confirm shared layout helpers, keyboard helper functions and existing screen styles were preserved. These are automated/source checks, not phone tests. Export emitted only the environment warning that FORCE_COLOR overrides NO_COLOR.

Pending physical Android checks: verify the EN pill cannot open anything, walk all forward/Skip/Back routes, confirm Ask script shaping and TalkBack labels, and test default/large/max system text, narrow/short/wide windows, safe areas, artwork crops and Login input/error/action reachability with keyboard open. No emulator or physical-device testing was performed.


## UI-only Register - October 26, 2026

Login Sign up now opens Register. Register Back and Log in return to a fresh Login; the original Welcome/onboarding Login origin is preserved. Android Back dismisses an open keyboard first, then returns Register to Login. This supersedes earlier notes that Login Sign up is disabled. Existing Login form/keyboard handlers and completed screens are unchanged.

Register has the six Figma fields: full name, National ID, email, phone, password and confirmation. All are required. The first four are checked only for nonblank input; example placeholders impose no format rules. Passwords are checked for nonempty raw values and exact equality, with no trimming or transformation. Strength, identifier-format and consent-validation rules await an agreed backend/content contract.

A visible notice says registration is disconnected. Local submission never creates an account, sends authentication/registration requests, simulates success, or opens verification/post-registration screens. Valid entries show only that no request was sent and no account was created. Terms of Service content/destination is missing: the inline link is visibly unavailable and accessibly disabled. The checkbox starts unchecked and is optional for this demonstration; nothing records consent. No Privacy link was added.

All form data stays in mounted component state, is never logged or persisted, and is discarded on exit. Explicit exits clear all native controls, values and visibility/checkbox state. Unmount cleanup captures mounted input controls before callback refs detach, so hardware navigation also clears native inputs. Leaving Login for Register unmounts Login and clears its credentials.

Screen/layout helpers, bundled Inter/Yaldevi/Noto Sans Tamil fonts and exactly verified Login artwork are reused. Eight original Register SVGs and native PNGs are bundled; source hashes/geometry are documented in [Register asset provenance](assets/figma/register/README.md). The revealed-password state reuses the existing Login eye icon. No dependencies, package/lockfile, backend files, or existing asset files changed. Expo remains 57.0.31.

The decorative header compacts before readable form text. Content is bounded to 480dp, safe areas are owned once by Screen, all interactive targets/fields are at least 48dp, labels/errors wrap, and one overflow ScrollView contains the header/card with its indicator hidden and overscroll disabled. No text scaling caps or fixed card clipping. Keyboard overlap adds only the part not already handled by a resized viewport. Form/vision measurements are intrinsic and guarded, avoiding header/card measurement feedback.

Validation: node scripts/check-register.cjs and TypeScript passed. Automated tests cover required fields, arbitrary identifiers without invented format rejection, Unicode names, raw/mismatched passwords, both visibility toggles, optional agreement, disabled Terms, notices, explicit exit and detached-ref unmount clearing; 18 synthetic layout/convergence cases and 36 resize/overlay keyboard cases across 320x568, 360x640, 360x800, 412x915, 600x960 and reference 400x772 with font scales 1/2/3; all six Login origins and Register Back routes; 12 original asset hashes. Existing file hashes and exact Login-only wiring changes were checked. PNG icons were visually inspected as assets, not rendered app UI.

Android production export passed with Hermes and 43 bundled assets. It emitted only the environment warning that FORCE_COLOR overrides NO_COLOR. No emulator or physical-device rendering was performed.

Pending Android phone checks: compare with Figma at reference size; inspect compact header/card, natural overflow, safe areas/system bars, field/placeholder wrapping, Unicode-name shaping and TalkBack focus/disabled Terms/checkbox announcements. Test the five Android sizes with normal/enlarged text, cutouts and both navigation modes. Exercise all six fields and keyboard Next/Done, first-invalid focus, errors and mismatch reveal, both password toggles, keyboard open/close/resize/overlay, action reachability, every exit and fresh re-entry. Confirm unchecked agreement permits local validation and there is no network/account/verification action.

## Account Created development preview

Run `npm.cmd run preview:account-created:dev` to start Expo Go with a process-local preview flag. Both __DEV__ and EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW=1 are required by App and the screen. Normal startup has no preview entry. Production ignores the flag; the production Android export inspected with flag=1 excludes the preview module and its icon. No APK build is involved.

All content is explicitly labelled as a sample: no account has been created. Rathishan is static Figma copy, not user data. Continue is disabled because its destination is unimplemented. Exit preview and Android Back return to Welcome. Register validation, submission, navigation and credential cleanup remain unchanged. The preview sends no requests and persists no data.

Validation commands:
- npm.cmd run typecheck
- node scripts/check-account-created-preview.cjs
- node -e "global.__DEV__=false;require('./scripts/check-register.cjs')"
- After production export: node scripts/check-account-created-preview.cjs --export

Guard/navigation and layout checks use mocked React/native measurements, not native rendering. Synthetic windows: 320x568, 360x640, 360x800, 412x915, 600x960, and reference 400x772; font scales 1, 2 and 3. Repeated intrinsic measurements converge in these mocks.

Pending Android phone checks: actual wrapping, measured-layout stability, default/enlarged text, safe areas, artwork crops, scrolling/footer reachability, TalkBack reading and disabled-control announcements, Exit/Back, and Login/Register keyboard and credential-cleanup regression. No emulator or physical-device verification is claimed.




## Forgot Password (UI-only)

Login's Forgot Password link opens the frame 399:2071 screen. It explicitly clears Login identifier/password/visibility/error state before leaving. Back and Log in return to a fresh Login while retaining the original Welcome/onboarding return destination. Android Back dismisses the keyboard first, then returns to Login; exit/unmount clears the NIC and validation state.

The NIC starts empty and is checked only for nonblank input. No format, length, checksum or account-existence rule is inferred from the Figma example. Description: Enter your NIC number. Password recovery is not connected in this UI-only preview. Support note: Recovery support details are not available in this UI-only preview. Nonblank submission stays on this screen and says: Password recovery is not connected. No request was sent and no password was changed. No requests, reset tokens/OTPs, storage, logging, fake progress or recovery destinations are implemented.

Three new original icons are preserved under assets/figma/forgot-password with provenance; verified shared backgrounds/logo/Back are reused. Fonts, responsive text, hidden-indicator overflow, safe-area ownership and 48-dp targets follow existing conventions. Decorative space reduces first; keyboard resizing does not replace the closed-screen baseline.

Validation: node scripts/check-forgot-password.cjs (includes existing Register/navigation regressions), node scripts/check-account-created-preview.cjs, npm.cmd run typecheck, and npm.cmd run export:android. The optional --export preview check requires a generated dist/android export. No EAS configuration or native APK build is involved. Synthetic measurements are not device rendering. Pending phone checks: actual wrapping/crops, enlarged text, keyboard resize/overlay/focus/error reachability, Back/Log in cleanup, TalkBack announcements and safe/system bars at narrow/short/tall/wide windows.


## Check Email development-only preview

Run npm.cmd run preview:check-email:dev. Optional port selection: npm.cmd run preview:check-email:dev -- --port 8082. The launcher uses supported Expo start --go --lan arguments and default development mode. It sets EXPO_PUBLIC_CHECK_EMAIL_PREVIEW=1 and Account Created preview=0 only in its child process, preserves the parent environment, and forwards the exit status. No env file is written. An existing Metro server must not be stopped without authorization; choose a free port instead.

The App entry and direct screen mount require __DEV__ plus the Check Email flag. Production ignores that flag and excludes the dynamically required module and its exclusive mail icon. Account Created keeps its own separate guards; if both flags are set manually, Account Created takes precedence and exiting it does not open Check Email. No link is added to Forgot Password submission.

The compact notice reads Preview - No email sent. The footer keeps Use a different NIC with no visible Exit preview control; Back has an in-bounds 48-dp target. The intrinsic card uses reduced padding/gaps, and the burgundy disabled email button retains its unavailable caption. The address is always sample@example.test. No reset link exists or expires, Open email app and Resend are visibly/accessibly disabled with no handlers, and no network, mailbox, timer, token or resend logic is implemented. Back, Android Back and Use a different NIC close the preview and open fresh Forgot Password; The static resend row reads Didn't get it? Resend, with an adjacent unavailable caption and accessibility disabled state. The normal Forgot Password Log in flow preserves Login origin; standalone preview begins with Welcome as its original destination.

Run node scripts/check-email-preview.cjs, node scripts/check-forgot-password.cjs, node scripts/check-account-created-preview.cjs and npm.cmd run typecheck. After Android export, add --export to both preview check scripts to inspect production bundle/assets. Tests cover flag combinations, direct mount, disabled actions, routes, launcher port/environment/status, and 18 synthetic logical-window/font-scale cases. They do not prove native rendering.

Pending phone checks: actual wrapping at default/enlarged text, safe areas/system bars, logo/background crops, compact notice/overflow/measurement stability, TalkBack disabled-control announcements and Back/Login cleanup. No EAS setup or APK build is involved.


## Reset Password development preview - October 27, 2026

Reference: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=399-2206

From app/mobile in PowerShell:

    npm.cmd run preview:reset-password:dev
    npm.cmd run preview:reset-password:dev -- --port 8083

The optional port must be free; do not stop an unrelated Metro instance. The
launcher uses the existing wrapper with start --go --lan and optional --port;
it does not pass --dev. It sets EXPO_PUBLIC_RESET_PASSWORD_PREVIEW=1 and both
older preview flags to 0 only in the child environment, without writing an env
file. Both App entry and direct mounting require __DEV__ and the exact flag.
Manually combined flags preserve Account Created then Check Email precedence;
Reset Password opens only when both older flags are off. Exiting an older
preview does not open this one. Normal Forgot Password/Check Email behavior and
normal navigation are unchanged.

The concise notice identifies a disconnected development preview and asks for
sample passwords only. Both fields start empty/masked, with independent Show/
Hide controls. Only nonempty values and exact matching are checked. Spaces and
case are never trimmed or normalized. Red outlines appear only for validation
errors. Matching nonempty samples stay on the form and state: Password recovery
is not connected. No request was sent and no password was changed. Editing clears
that submission notice. No reset tokens, deep links, requests, storage, logging,
password change or simulated success are implemented.

There is no visible Back/Exit control. Android Back dismisses an open keyboard
first; the next Back clears native inputs, values, visibility, errors and notice,
then returns to Welcome. Unmount clears native inputs and removes keyboard/Back
listeners and layout timers; React values stay local to the mounted form.

Approved adaptations: four neutral strength bars, Strength not evaluated, neutral
rule markers and guidance marked pending backend confirmation replace strength/
satisfied-rule claims. The four Figma rule strings and introductory guidance are
not enforced policy. Backend must confirm strength/length/reuse rules, reset-token
and expiry handling, and actual reset destinations/states.

Existing verified Login/Register ribbons, skyline, logo, lock and eye assets are
reused unchanged. The logo keeps the existing nested crop (154.17% width,
271.32% height, -30.13% left, -63.6% top), with a maximum 118px slot width.
The reference 400px viewport has a 368px card. Inter text, rounded white card and
burgundy CTA follow the design. Notice/pending copy, 48dp controls and scalable
text can make the card taller than the reference 505px; its height is intrinsic.
Decoration yields before readable text. Screen owns safe areas, wide content is
bounded, and overflow scrolling has hidden indicators. Android owns real system
chrome. Keyboard handling follows Forgot Password's closed-viewport baseline;
only overlap remaining after native resize is added. Automatic keyboard/content
insets are disabled; no KeyboardAvoidingView duplicates compensation. Focus
adjustment runs only when a field/error group is outside the visible region.
Inputs, errors and the action stay in normal scroll flow; no entry-time forced
scroll or typography shrink is used.

Repeat checks from app/mobile:

    node scripts/check-reset-password-preview.cjs
    node scripts/check-forgot-password.cjs
    node scripts/check-email-preview.cjs
    node scripts/check-account-created-preview.cjs
    npm.cmd run typecheck
    npm.cmd run export:android
    node scripts/check-reset-password-preview.cjs --export dist/android
    node scripts/check-email-preview.cjs --export
    node scripts/check-account-created-preview.cjs --export

For the flag-on production check, use a dedicated PowerShell shell or preserve
and restore any existing variable:

    $env:EXPO_PUBLIC_RESET_PASSWORD_PREVIEW = '1'
    node scripts/expo.cjs export --platform android --output-dir dist/reset-preview-flag-on
    node scripts/check-reset-password-preview.cjs --export dist/reset-preview-flag-on
    Remove-Item Env:EXPO_PUBLIC_RESET_PASSWORD_PREVIEW

Actual coverage: TypeScript and all existing form/navigation/preview checks
passed. New mocked checks passed 32 flag/development/precedence cases, direct
guard, required/raw mismatch cases, independent toggles, keyboard-first Back and
clearing, plus 18 synthetic viewport/font-scale cases: 320x568, 360x640, 360x800,
412x915, 600x960 and reference 400x772 at 1x/2x/3x. They check convergence,
neutral labels, 48dp control heights, hidden-indicator overflow and overlay vs
resized-keyboard compensation. These are simulations, not native screenshots.
An optional task-time .cache hash snapshot verifies existing screen and lockfile
preservation; the new script also works without that snapshot on a fresh checkout.

The actual launcher started Metro on port 8083. Its /status endpoint returned
HTTP 200 and packager-status:running; an Android development bundle returned
HTTP 200 containing the preview. Only that verification launcher and its child
processes were stopped afterward. Expo used a fallback after a DevTools-download
warning. Actual production Android/Hermes exports passed with the preview flag
unset and set to 1. Both Hermes bundles passed exclusion checks; neither contained the
preview module name or unique preview copy. Existing preview module/asset
exclusion checks passed too. An initial sandbox denial of the existing Hermes
executable was resolved by running export with execution permission; no install.

Pending physical Android checks: Expo Go launch, compare at reference 400x772,
default/enlarged/max text, narrow/short/tall/wide windows, safe areas, artwork
crops, actual keyboard resize/overlay, Next/Done, errors/action reachability,
conditional scrolling, TalkBack toggles/error announcements, keyboard-first Back
then Welcome, and fresh empty/masked values on preview relaunch. No emulator,
physical-device or native visual comparison was performed for this screen.


## Password Updated development preview - October 27, 2026

Reference: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=399-2303

From app/mobile in PowerShell:

    npm.cmd run preview:password-updated:dev
    npm.cmd run preview:password-updated:dev -- --port 8084

The optional port must be free. The launcher uses supported start --go --lan
arguments plus optional validated --port, never --dev. It sets
EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW=1 and Account Created/Check Email/Reset
Password flags to 0 only in the child process. No environment file is written.
Existing Metro instances are not stopped. Entry and direct mounting both require
__DEV__ plus the explicit flag. With manually combined flags, older previews
retain their precedence; this preview opens only if all three older flags are
off. Exiting another preview cannot open Password Updated afterward.

The exact notice is Preview · No password was changed, above the card and in
its scroll flow. The reference heading remains Password updated; approved body:
This is a design preview. No password was changed and no devices were signed out.
There are no fields, tokens, requests, storage, automatic redirects or actual
password/session changes. Reset Password submission stays disconnected and does
not enter this screen. The single visible Log in button closes this preview and
mounts a fresh Login with Welcome as its Back destination. Android Back directly
returns from the preview to Welcome. There is no visible Exit/Back control.

The actual shield-check artwork is exported from Figma, not substituted with the
Account Created icon. Provenance, hashes, reused asset matching and geometry are
in assets/figma/password-updated/README.md. All five shared source downloads
(ribbons, skyline, logo and arrow) matched the existing originals by SHA-256.
The new 136x136 transparent PNG displays in the reference 34x34 slot, centered
inside its 72x72 badge. The card preserves the 368px reference width at 400px;
24px padding, 16px vertical gaps, 20px heading, 13px body, 16px action text and
burgundy action follow the reference. Approved notice/body, scalable wrapping
and >=48dp controls can increase the intrinsic card beyond its 271px reference.
The 150x151.531 logo crop retains existing ratios. Decorative header space yields
before text, wide content is bounded, Screen owns safe areas and natural overflow
scrolling has both indicators hidden. Native Android owns system chrome.

Repeat checks:

    npm.cmd run typecheck
    node scripts/check-password-updated-preview.cjs
    node scripts/check-reset-password-preview.cjs
    node scripts/check-forgot-password.cjs
    node scripts/check-email-preview.cjs
    node scripts/check-account-created-preview.cjs
    npm.cmd run export:android
    node scripts/check-password-updated-preview.cjs --export dist/android
    node scripts/check-reset-password-preview.cjs --export dist/android
    node scripts/check-email-preview.cjs --export
    node scripts/check-account-created-preview.cjs --export

For a flag-on production check, use a dedicated PowerShell shell or preserve
and restore the previous variable value:

    $env:EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW = '1'
    node scripts/expo.cjs export --platform android --output-dir dist/password-updated-flag-on
    node scripts/check-password-updated-preview.cjs --export dist/password-updated-flag-on
    Remove-Item Env:EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW

Actual checks: TypeScript and all existing form/navigation/preview regression
checks passed. New checks passed 64 flag/development/precedence cases, direct
mount guards, Log in and Back destinations, unchanged Reset Password submission,
one visible control, isolated launcher flags/port/status, rejection of --dev and
invalid arguments, exact icon hash, plus 18 synthetic layout/convergence cases.
The cases cover 320x568, 360x640, 360x800, 412x915, 600x960 and reference 400x772
at 1x/2x/3x text. Optional task-time cache hashes verified other existing screens
and the lockfile were unchanged. These mocks check logic and geometry, not native
rendering or screen-reader behavior.

The actual launcher started Metro at port 8084; /status returned HTTP 200 with
packager-status:running, and an Android development bundle returned HTTP 200
containing the preview. Expo reported a DevTools-download warning and used its
fallback. Only this verification launcher and its children were stopped after
testing. Both actual production Android/Hermes exports succeeded (new flag unset
and set to 1); module name, unique preview text and shield PNG hash were absent.
All older preview production-exclusion checks also passed on the new export.
No dependency install, backend modification, firewall change or EAS work occurred.

Pending physical Android checks: launch in Expo Go, compare reference 400x772,
check narrow/short/tall/wide layouts, default/enlarged/max text, safe areas, logo
and ribbon crops, natural overflow, TalkBack notice/heading/button announcements,
Log in with empty fields and Login Back to Welcome, and Android Back from this
preview to Welcome. No native screenshot, emulator or physical-device rendering
was performed for this screen.
