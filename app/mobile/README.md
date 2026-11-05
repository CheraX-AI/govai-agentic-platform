# GovAI mobile

## Repository availability

The frontend screens, navigation, original Figma assets and guarded development
launchers are included in the repository. Authentication, account recovery and
chat still contain explicitly labelled development-only simulations; they do not
authenticate users, send recovery messages or contact an AI service. Real
backend/API integration, verified identity data, secure sessions and production
chat behavior are still required.

Optional mock regression and served-bundle diagnostic scripts are kept only in
the development maintainer's local checkout and are excluded from Git. A GitHub
clone therefore should use the shared `npm.cmd run typecheck`,
`npm.cmd run check:expo`, preview launchers and Android export commands documented
below; it should not expect the historical local checker commands to exist.

## GA-20 and GA-22 integration — November 5, 2026

All screen components live directly in `pages/`; reusable presentation stays in
`components/`, while entry points, configuration and shared layout helpers remain
at the app root. The integration preserves the GA-20 authentication, registration,
onboarding and Email/Phone recovery screens together with the GA-22 Chat Home,
drawer and local conversation experience.

Normal authentication, recovery and chat remain honestly disconnected. With both
`__DEV__` and `EXPO_PUBLIC_CONNECTED_UI_DEMO=1`, locally validated Login opens Chat
Home, Register opens Account Created, and Forgot Password opens Check Email or
Check Phone according to the selected method. Standalone previews keep their own
explicit development flags. These routes do not authenticate a user, deliver an
email/SMS, verify a code, persist a session or call an AI/backend service, and the
development-only modules remain excluded from production bundles.

The optional local regression and served-bundle diagnostics listed in `.gitignore`
are not repository dependencies. Operational preview launchers and icon
rasterization scripts remain tracked because the frontend documentation and asset
workflow use them.

## Recovery demo launcher and live-bundle verification — November 4, 2026

Use `npm.cmd run demo:connected-ui -- --port 8081` for the connected recovery
demo. Reopen that server in Expo Go after changing launch modes; do not continue
using an older loaded session. Plain `npm.cmd start` intentionally keeps recovery
disconnected unless its environment is explicitly changed. The connected launcher
sets EXPO_PUBLIC_CONNECTED_UI_DEMO=1, now clears Metro's cache on startup, and
prints the selected mode. This does not enable recovery in normal or production
mode and does not change any screen, validation rule, callback or route guard.

The reported Phone-unavailable notice was reproduced from an actual normal-mode
Android bundle (flag 0). An actual connected-launcher bundle (flag 1) passed Phone
and Email submission-to-route tests: empty NIC and unchecked simulation remain
on the form; valid Phone opens CheckPhoneDemoScreen; valid Email opens Check Email;
Verify code remains disconnected; all return controls and keyboard-first Back
preserve fresh forms and Login origin. There was no local Metro listener before
inspection, so the exact bundle previously loaded on the phone could not be
identified; wrong-mode or stale client state remains the likely explanation.

A local-only served-bundle diagnostic previously verified the actual Expo
environment injection and executed App and recovery-form factories with mocked
React hooks/native controls. That optional diagnostic and its bundle captures
are excluded from Git; its result did not establish physical-device rendering or
an Expo Go device connection.

## Check Phone connected demo — November 4, 2026

Phone now opens the single new pages/CheckPhoneDemoScreen.tsx only after Continue
passes the existing nonblank-NIC and simulated-checkbox checks, with both __DEV__
and EXPO_PUBLIC_CONNECTED_UI_DEMO exactly enabled. Only the selected method is
passed to navigation; the NIC is cleared and never used for a destination lookup.
Typing does not navigate. Email retains Check Email. Normal disconnected Email
and Phone submission notices are unchanged. This supersedes older statements
that Phone is always unavailable.

The screen uses original GovAI artwork and a composed, green-tinted Register
phone / Chat Home message badge. No image bytes, dependencies, system bars or
status-bar settings were changed. It displays “Demo preview — no SMS was sent”
and the fixed masked sample +94 7••• ••12. The six visual slots represent one
accessible native numeric field, not six separate screen-reader controls.
They share a full-width target of at least 48dp height and hide their duplicate
visual text from accessibility. Digits and other text retain system scaling.

Manual numeric typing, native deletion/selection and pasted digits are supported.
Spaces/hyphens in a paste are removed. Letters or more than six digits are
rejected without truncating or replacing the prior value. An incomplete field
shows a local error. Verify code has exactly that visible label; every complete
six-digit sample produces “Verification is not connected. No code was verified.”
There is no successful code, authentication result or onward navigation. Six
digits are a proposed UI-only format; backend length, expiry and attempt rules
remain unconfirmed. No OTP/resend timer, SMS read/send, token, storage or automatic
clipboard access is added. Existing keyboard-settling timeouts only manage layout.
Resend code is visibly/accessibly disabled and has no handler.

Header Back and Use a different NIC clear the code and return to a fresh Forgot
Password form. Android Back first dismisses an open keyboard and retains the
draft, then returns to cleared Forgot Password; unmount clears the native field.
Back to log in clears the code and opens fresh Login without changing loginOrigin.
The standalone previews and their precedence remain unchanged. The Phone module,
demo copy and exclusive message image are excluded from production exports;
the shared Register phone image may legitimately remain in production.

Validation: TypeScript, Expo compatibility and all eleven regression scripts
passed. Phone checks execute local input/rejection, no-authentication, repeated
exit cleanup, guard combinations and 18 mocked layout/keyboard cases; connected
tests cover actual Phone controls, keyboard-first Back, both recovery methods
and all six Login origins. The corrected browser harness covers six sizes with
text scales 1/2/3 plus error, filled, notice and short-window cases (25 total).
It measures a 497px card at 400 × 772 and 513px at 360 × 800, fitting normal
state in their simulated safe-area viewports. Normal state at 320 × 568 has
159px overflow and at 360 × 640 has 127px; scrolling remains available. Enlarged
text/errors also scroll, with every control reachable and no horizontal overflow.
Screenshots of empty/filled reference and narrow/enlarged cases were inspected.
These are HTML and mocked-native measurements, not Android-device evidence.
Physical Android fit, IME resize/overlay, native paste/selection/Back behavior,
system font scaling and TalkBack acceptance remain pending.

## Check Email reference update — November 4, 2026

The existing Check Email screen now follows the supplied reference with the
original GovAI logo/crop, ribbons, skyline, Back arrow, mail/check icon, info
icon and white action arrow. The original blue mail PNG is tinted green through
React Native styling; image files are unchanged. The width-safe logo slot is
150 × 151.531dp at 400dp width. The centered white card has intrinsic height,
16dp padding, a 64dp green-tinted badge, readable/scalable text, a blue help
panel and a burgundy Back to log in button. Resend link is visibly unavailable,
accessibly disabled and has no handler. The notice says “Demo preview — no email
was sent.” The address remains the static sample@example.test, not an account
lookup. No request, mailbox launch, timer or real recovery has been added.

Email + nonblank NIC + the simulated checkbox + Continue opens this screen only
in the existing guarded connected-development mode. Typing, invalid submissions,
Phone and normal disconnected mode do not navigate to it. Back to log in opens
a fresh Login without changing loginOrigin. Header Back, Use a different NIC
and Android Back return to a fresh Forgot Password form. Both the standalone
and connected wrappers retain their independent development flags and production
exclusions. No duplicate screen, simulated phone chrome or home-indicator
decoration was added; native system bars, safe areas and status-bar handling
remain unchanged. The original logo's opaque white background is retained.

TypeScript, Expo compatibility and all ten regression suites passed, including
the actual login-button callback through the wrapper/presentation and all six
Login return origins. Corrected constrained-browser simulations cover both modes
at six sizes with text scales 1/2/3, plus two short-window cases (38 cases).
With simulated 24dp top/bottom safe insets, the normal card measures 435px and
fits at 400 × 772, 360 × 800 and 412 × 915. At 320 × 568 there is 97px of
overflow; at 360 × 640 there is 65px. Scrolling remains available and every
control is reachable with targets of at least 48dp. The normal reference and
narrow/enlarged screenshots were visually inspected. These are HTML simulations,
not native Android or TalkBack tests. Physical-device fit, keyboard/system-font
behavior and screen-reader acceptance remain pending. This section supersedes
older Check Email layout and disabled-mailbox-button notes below.

## Recovery form spacing refinement — November 4, 2026

The existing screen uses explicit spacing instead of one card-wide 6dp gap:
badge to title group 8dp; heading to description 4dp; description to recovery
group 10dp; radio group to NIC group 10dp; input to checkbox group 8dp; checkbox
to simulation caption 4dp; caption to Continue 8dp; Continue to login target 6dp.
Label/control gaps remain 4dp. Card padding remains 16dp horizontally and 12dp
vertically, with no fixed height. Logo/header separation remains 8–12dp.

The corrected, constrained browser simulation measures a 492px card at
400 × 772 and 360 × 800, and 474px at 412 × 915. These normal-text cases fit
without scrolling using the simulated 24dp top/bottom safe-area insets and
existing Screen padding. At 320 × 568, normal content is 640px in a 488px
viewport (152px overflow); at 360 × 640 it is 662px in 560px (102px overflow).
Scrolling remains available. At 400 × 772, empty-submit errors require 35px
of scrolling and the Phone notice requires 33px. No text or content was clipped
or compressed. All 23 browser cases passed; these are HTML measurements, not
native-device fit. Android fit, IME behavior, system font scaling and TalkBack
remain pending device testing.

The larger logo/crop, source image's opaque white background, wording, font
sizes, 48dp targets, application logic and status-bar behavior are unchanged.
This section supersedes the earlier uniform-gap spacing and card measurements.

## Recovery form fit correction — November 4, 2026

The existing screen now uses the subtitle “Enter your NIC number and choose a
recovery method.” The yellow banner and sample-destination line are removed.
The Demo check tag, simulated/no-security-verification caption, exact Continue
label, validation, Email demo guards, Phone unavailable/no-SMS notice and normal
disconnected submission notice remain. Status-bar behavior is unchanged.

The original logo crop is retained in a width-safe slot: 150 × 151.531 dp at
400dp width, 130 × 131.327 dp at 360dp, and 90 × 90.919 dp at 320dp. Card growth
and keyboard transitions no longer shrink the logo. Card padding is 12dp
vertically, section gaps 6dp and the decorative badge 40dp. Text sizes and font
scaling are unchanged; interactive targets remain at least 48dp. There is no
fixed card height or content clipping; overflow scrolling remains enabled with
its indicator hidden.

Corrected browser simulation measurements (24dp top/bottom safe-area insets,
plus the existing Screen padding; normal text, keyboard closed):

| Viewport | Available content height | Intrinsic card height | Scrolling |
| --- | ---: | ---: | --- |
| 400 × 772 | 692px | 474px | No |
| 360 × 800 | 720px | 474px | No |
| 412 × 915 | 835px | 456px | No |
| 360 × 640 | 560px | 474px | Yes |
| 320 × 568 | 488px | 492px | Yes |

The earlier HTML harness allowed the scroll container to expand; its scroll/fit
results must not be used as evidence. The corrected harness explicitly bounds
the scroll viewport, asserts its safe-area position/height and checks every
control can be revealed. All 18 viewport/text-scale cases and five additional
error/Phone/short-window cases passed. Reference/narrow screenshots were inspected.
These are measured HTML simulations, not native Android measurements. Actual
phone fit, Android IME resize/overlay behavior, system font scaling, TalkBack and
safe/system bars remain pending device testing. This section supersedes the
older always-visible-banner, sample-destination and shrinking-logo notes below.

## Project organization

Screen components live directly in `pages/`; reusable UI stays in `components/`.
`assets/` holds artwork, fonts, licenses and provenance. Development launchers,
validation and generation scripts live in `scripts/`. Shared `layout.ts` and
`chatDemo.ts`, entry points and configuration remain at the app root.

Historical screen filenames in earlier validation records now resolve under
`pages/`. Optional source audits reverse only the exact approved import-path
substitutions before comparing original hashes; other content changes still fail.
The mapping is retained by an optional local historical audit. For moved screens only,
the audit reverses `'../components/` to `'./components/`, `'../layout'` to
`'./layout'`, and `'../assets/` to `'./assets/`. Historical snapshot files,
hashes and pre-existing exclusions are retained.

Organization validation (November 4, 2026): TypeScript, Expo compatibility, all
ten regression scripts, and both Android exports passed. All eight artifact
exclusion suites passed against exports with all six demo/preview flags off and
on. Source-byte comparisons confirmed only approved module-path edits in app
code; all 177 existing asset files and package/lockfile/ignore/instruction bytes
were preserved. Negative audit fixtures rejected unexpected source changes.
These checks include mocked native APIs, not physical Android or TalkBack testing.

## Official website reference - October 28, 2026

The approved NIC sample reference is now a link to the fixed homepage
`https://drp.gov.lk/`. Visible text is exactly "Official website · Demo reference,
not verification of this response". This supersedes earlier documentation saying
the reference is noninteractive/pending implementation. It does not verify the mock
reply, its sample steps or service details, and no checked date is claimed.

OfficialSourceReference.tsx owns the reference interaction. ChatMessage.tsx replaces
its plain source row with this component using the existing globe. The link has a
minimum 48x48dp target, wrapping/scalable underlined burgundy text, link semantics,
a department-specific accessibility label including the disclaimer, and an external
browser hint. Unimplemented service actions remain disabled. Other notices, chat
navigation/draft/scroll state, keyboard handling and production guards are unchanged.

Only an explicit tap calls React Native Linking.openURL with the constant homepage.
No chat text, credentials, identifiers, query parameters or fragments are passed.
The app does not fetch the website or embed a browser. Android handles the URL in
its external handler/browser or chooser. Repeated taps are blocked while opening.
Handoff rejection or a synchronous failure produces the accessible inline alert
"Couldn’t open the official website. Please try again." The link is available for
retry; a new attempt clears the error. Late completion after unmount cannot update
state or reopen anything. Subsequent network/site errors belong to the browser;
a successful handoff does not mean the website loaded or verified the response.

The five-file scope is OfficialSourceReference.tsx, ChatMessage.tsx, the Chat Home
fixture and conversation checker, and this README. Linking is mocked in automated
tests; those tests never open the browser or contact the government site. Coverage
includes exact URL/disclaimer, link role/target, success, rejected/thrown opening,
successful retry, duplicate taps, late/unmounted callbacks and both entry modes'
conversation/draft/scroll retention. TypeScript, drawer/connected navigation and
conversation regressions accompany flag-on/off production exclusion checks for
the new module, disclaimer and URL. Production artifacts use
dist/source-reference-production-1 and dist/source-reference-production-0.

Pending Android device checks: activate the NIC reference in standalone/connected
demo, verify the external browser/chooser and exact homepage URL, then return with
browser Back and confirm the conversation/draft/reading position. Test real browser
handoff failure where reproducible, retry, TalkBack label/hint/error announcement,
48dp reachability and enlarged text. No device verification is claimed. The Jira
source-link interaction is implemented locally; Android acceptance remains pending.
The mock content stays unverified and the status-bar issue remains deferred.

## GA-22 local conversation demo - October 28, 2026

Implements conversation frame 97:26 inside the existing guarded Chat Home owner.
Both explicit development entry modes support it; normal Login remains disconnected
and all chat code/assets remain excluded from production. App.tsx and pages/LoginScreen.tsx
are unchanged by this task. This section supersedes older statements that landing
suggestions, Send and drawer New chat are disabled. Other destinations stay disabled.

Shared ChatHeader, ChatComposer and ChatBottomNavigation retain landing presentation
and provide conversation variants. ChatMessage exports CitizenMessage and AiResponse;
ChatConversation owns list scrolling. chatDemo.ts contains typed local fixtures and
conversation assets. No network transport, authentication, persisted history or
credential storage exists. The local AGENTS.md context now includes these frames
and retains its Git exclusion and all safeguards.

Suggestion taps send exactly that prompt and preserve an unrelated landing draft.
Landing Send rejects blank/whitespace with accessible "Enter a message." Valid Send
clears the submitted draft and starts a local thread. Follow-up Send appends one
citizen message, clears the composer and prepares a canned response after 800ms.
Draft typing stays available during responding; Send is disabled until responding
or its recoverable error is resolved. Synchronous request/version guards reject
duplicate and stale submissions. Retry reuses the citizen message and prepares
one response. The development-only "Simulate next response error" switch sits in
a labelled Demo controls strip outside the conversation content; it fails the next
response once. Pending text and error explicitly say "local mock response".

All responses are marked "Local demo · Mock response". The visible notice states
"Local demo · Responses are samples, not verified government advice. AI is disconnected."
Connected mode also retains the credentials-not-verified notice. The exact NIC
suggestion receives a sample five-step layout and sample service card. Every step
says "demonstration only"; card values say "Not verified". Other prompts receive
a generic mock response without an unrelated NIC card or invented source. The
original Figma fee, duration, actionable instructions and verification date are not
presented as guidance. No AI streaming or server success is simulated.

**GA-22 Jira source-link requirement: pending.** "Design reference: drp.gov.lk ·
Not verified" is plain, noninteractive text. No hyperlink, checked date or verified
advice claim is implemented. An approved verified destination and external-browser
interaction still need definition before enabling source links. Start renewal,
Ask next chips, attachment, microphone, Services, My Requests and Profile remain
accessibly disabled. Figma destinations 114:2, 230:76 and 186:464 are outside scope.

Conversation Back and Home dismiss the keyboard, cancel pending work and clear the
thread/follow-up draft, returning to landing with an unrelated unsent landing draft
preserved. Header/drawer New chat cancels work, resets all chat drafts and error-test
state and opens empty landing. Drawer remains available on landing only, matching
the conversation's Back/New chat header. Its fixed footer, overflow fallback,
background isolation and focus restoration are retained. Logout from landing clears
all remaining chat state and returns to fresh Login with its original Back origin.
Standalone Logout stays disabled. Android Back closes a drawer first, dismisses
keyboard next, then conversation -> landing -> existing Login/Welcome destination.
Unmount also cancels response timers; stale results cannot recreate departed threads.

The conversation list follows new content only when within 96dp of the bottom.
Reading older messages retains position; new messages expose a 48dp Jump to latest
action. Keyboard overlap resizes the conversation shell; when controls overflow,
only the outer shell scrolls to expose the composer, preserving the inner history
offset. Header/composer/navigation and the demo strip use
intrinsic measured heights, with at least 96dp for list/Jump controls. Short windows
or enlarged text allow outer-shell overflow so fixed controls stay reachable.
Existing safe-area ownership and status-bar handling remain unchanged; the unresolved
Android top-status-bar issue is still deferred. Text scales/wraps; controls have
minimum 48dp targets and images explicit dimensions. Source/crop/hash details are
in assets/figma/chat-conversation/README.md.

Shared repeatable checks are `npm.cmd run typecheck`, `npm.cmd run check:expo`
and `npm.cmd run export:android`. Optional local regression coverage uses
mocked native APIs for both modes, prompt/typed/blank/follow-up input, responding,
recoverable error/Retry, duplicates, cancellation and stale results, drafts,
Back/New chat/Logout, scroll retention/Jump, disabled services/source and 15 synthetic
viewport/text cases. Asset hashes and reused PNG equality were checked locally.
Optional local browser approximations did not prove Android rendering.
Android/Hermes exports with all flags enabled and disabled were also inspected
locally for guard behavior.

Launcher verification uses `npm.cmd run preview:chat-home:dev -- --port 8098` on a
free port; /status and Android development bundle must return HTTP 200 and contain
the conversation/demo controls. Existing user Metro on 8097 is preserved. Expo may
report its existing DevTools-download fallback warning; no dependency install is needed.

Actual results: TypeScript, conversation/menu/connected-demo and all existing form
and confirmation regression suites passed. Both flag-on/off Android/Hermes exports
passed conversation, Chat Home/menu, connected-demo and older preview exclusion
checks. Browser approximations at reference, narrow, wide and 2x text were inspected,
including service-card/source/footer content. Launcher 8098 started, /status and its
Android bundle returned HTTP 200 with conversation/demo-control markers. Only that
verification server was stopped afterward; 8097 still returned HTTP 200. These are
automated/build/browser checks, not Android device tests.

Pending physical Android tests: open standalone and connected demo; exercise all
suggestions and typed messages, whitespace errors, pending-draft typing, simulated
error/Retry, Back/New chat/Logout during pending work, fresh Login and original
Welcome/onboarding origins. Scroll up before a response: position must stay put and
Jump to latest must work. Test actual keyboard resize/overlay, short landscape and
multiwindow, enlarged/max text and full service-card/chip reachability. With TalkBack,
verify errors/responding announcements, reading order, source noninteraction, disabled
actions and drawer focus restoration. Check avatar crop and Android safe areas.
Automated callbacks/calculations/browser approximations are separate from these
unperformed native/device checks. Source-link completion and status-bar remediation
remain pending; neither is claimed complete.

## Fixed drawer header/footer - October 28, 2026

The drawer normally keeps its logo/Close header and Citizen/Logout footer outside
the scrollable menu/history middle. This supersedes earlier whole-drawer scrolling
descriptions. The shell owns top inset + 4dp, bottom inset + 8dp, left inset + 20dp
and 20dp right padding; existing drawer width leaves the right safe area outside.
Insets are not duplicated on the middle or fixed regions. The footer has natural
height, stays above the bottom safe area and includes the existing Logout notice.

Compact spacing: header bottom 8dp, primary rows minimum 48dp with 10dp vertical
padding, middle gaps 4dp, history groups 8dp apart, history row vertical padding
8dp with minimum 48dp height, Citizen top padding 8dp. The decorative flexible
spacer is removed. Existing text sizes, system scaling, wrapping, images and
minimum targets remain unchanged; no whole-interface scaling was added.

Intrinsic header/footer and actual drawer heights are measured. If those regions
plus safe-area padding cannot leave 48dp for the middle, the same mounted shell
scrolls the whole drawer and the inner scroll is disabled. Switching uses the same
header/footer/control tree and focus refs. The fixed-region measurements do not
depend on the flexible middle height. Taller windows or smaller text restore
middle-only scrolling after native remeasurement. This fallback keeps all content
reachable when fixed regions themselves cannot fit.

Only ChatHomeMenu.tsx, the menu/preview checkers and this README changed for this
layout task. ChatHomeContent, Logout cleanup, App navigation, Back priority, draft
retention, disabled destinations, production guards and deferred status-bar issue
are unchanged. TypeScript, menu and connected-demo regression checks cover these
behaviors. Menu checks simulate 21 viewport/font-scale cases, intrinsic measurement
changes, scroll ownership/recovery, footer placement, safe areas and preserved
font sizes/targets. Browser approximations cover 400x772, 320x568, 400x772 at 2x,
600x960 and an extreme 320x240 at 3x. They do not establish native Android layout.
Production Android/Hermes flag-on/off builds and exclusion checks use
dist/drawer-layout-production-1 and dist/drawer-layout-production-0.

Pending phone checks: confirm header/footer remain visible while scrolling history;
check whether normal-size content fits on the actual phone; verify Logout without
scrolling the normal drawer. Test short landscape/multiwindow and maximum text:
all middle items must remain reachable, and if fixed regions cannot fit the whole
drawer must scroll to Logout and back to Close. Recheck TalkBack focus after opening,
closing and layout changes, safe areas with gesture/three-button navigation, draft
retention and Logout/Back. No physical-device or emulator test was performed.

## Drawer Logout - October 28, 2026

The drawer now includes an outlined burgundy **Logout** action below the Citizen
footer, with "UI demo only · Returns to Login." It is enabled only in the guarded
connected development demo. The standalone preview disables it accessibly and shows
"Unavailable: no login occurred in this preview." This supersedes the earlier
menu section's statement that only Close and the backdrop are active.

Logout cancels pending menu animation/focus and keyboard-layout work, closes the
drawer, clears the native input and local draft, resets temporary menu/keyboard
state and dismisses the keyboard before calling the existing guarded Chat exit.
A synchronous leaving latch rejects duplicate, departed and unmounted callbacks.
Fresh Login retains its original Welcome/onboarding Back destination; Back cannot
reopen the logged-out Chat Home. There is no persisted session or server-side logout.
Normal X/outside/Back drawer closing still retains the draft and returns focus.
Production guards, other screens, safe-area ownership and pending status-bar work
are unchanged. No new assets, dependencies, backend calls or storage are involved.

Validation: TypeScript; Chat Home/menu and connected-demo regressions; existing
registration, recovery and confirmation suites; Android/Hermes exports with all
preview/demo flags enabled and disabled and their production-exclusion checks.
Logout checks cover disabled standalone behavior, cleared state before exit,
duplicate actions, pending focus cancellation, callbacks after unmount and after
later Login visits, fresh Login, and all six original Back destinations. Synthetic
layout checks include the new 48dp action and natural overflow at enlarged text.

Pending phone tests: use dummy Login inputs, type a draft, open the menu and tap
Logout; verify fresh empty Login, dismissed keyboard and Back to the original
Welcome/onboarding screen. Log in again and verify an empty draft/closed drawer.
Check rapid Logout taps, ordinary drawer close/draft retention, disabled standalone
Logout, TalkBack focus/disabled announcements, and scrolling to Logout on short,
landscape and enlarged-text screens. These are not claimed as device-tested.

## Chat Home menu - October 28, 2026

Figma frame 306:1388, drawer instance 306:1501. This section supersedes older
statements that the Chat Home menu control is disabled. The existing standalone
and connected-demo guards still apply; normal Login and production stay disconnected.

`components/ChatHomeMenu.tsx` is an in-tree overlay owned by
`components/ChatHomeContent.tsx`, not a navigation destination or native Modal.
The top-left button opens it after assets load. Opening blurs the composer and
dismisses the keyboard without clearing its local draft or remounting Chat Home.
Background touches and accessibility descendants are blocked while open, including
the closing fade. X, outside tap and Android Back close it; another Back follows
the existing keyboard-first/exit behavior. Close receives accessibility focus on
layout; closing restores focus to the opener without focusing the input. Reduced
motion skips the fade. TalkBack and actual keyboard timing need device verification.

The drawer follows the white 340dp reference panel, 24dp right corners, 55% scrim,
original brand/icons, primary rows and grouped Today/Yesterday history. Width
shrinks on narrow screens, retaining at least 48dp outside area; drawer content
owns its safe-area insets and scrolls naturally when text/content grows. Main Chat
Home inset ownership and the deferred status-bar issue are unchanged. Targets are
at least 48dp; artwork uses explicit dimensions and contain sizing.

Visible sample label: "Sample chat history · Actions unavailable". Footer:
"Citizen" / "Development preview", with the existing neutral profile icon.
New chat, Recent chats, My applications, all five sample history rows and the
profile/settings row remain accessibly disabled. Ellipsis icons have no separate
action. No account, chat creation, backend, authentication or storage was added.
Figma prototype links describe future destinations only: New chat/Close/scrim
to 77:4, Recent chats to 308:1734, My applications to 890:5808, history to 97:26,
profile to 186:464. Ellipsis/settings have no established separate destinations.
Only approved Close/scrim behavior is implemented.

Assets/provenance: `assets/figma/chat-home-menu/README.md` and `sha256.json`.
`scripts/rasterize-chat-home-menu-icons.ps1` reproduces native PNG icons from the
original SVGs using existing Chrome/System.Drawing; no runtime dependency added.

Shared checks are `npm.cmd run typecheck`, `npm.cmd run check:expo` and Android
export. Optional local registration, recovery, confirmation, connected-demo and
menu regression coverage
covers both entry modes, retained input identity/draft, keyboard dismissal,
X/outside/Back, background isolation, focus restoration, repeat cycles, reduced
motion, nine disabled destinations, asset hashes and 18 synthetic layouts.
Optional local tooling creates browser approximations
at 400x772, 320x568, 400x772 with 2x text and 600x960; these are not native renders.
Production Android/Hermes exports with all preview/demo flags on and off were
inspected locally for menu, Chat Home, connected-demo and preview exclusions.

Actual launcher verification: `npm.cmd run preview:chat-home:dev -- --port 8098`
started Metro; /status and an Android development bundle returned HTTP 200,
including menu/sample copy. Only that verification server was stopped; the existing
8097 development server was preserved. Expo used its DevTools-download fallback.

Pending physical Android testing: open both standalone and connected-demo entries;
type a dummy draft, open the menu, verify keyboard dismissal and unchanged draft;
close independently through X/outside/Back and verify background cannot be tapped.
With TalkBack, verify Close focus, hidden background and return to the opener.
Test narrow/short/landscape screens, cutouts/navigation insets, default/enlarged/max
text, overflow to Yesterday/footer, reduced motion and rapid open/close. Then check
existing Chat Back to fresh Login/Welcome and credential/draft cleanup on exit.
No physical-device, emulator or native accessibility/rendering verification was
performed for this menu. The unresolved Android status-bar issue remains pending.

## GA-55 Login Figma alignment - November 4, 2026

Login is aligned to the live Figma `Display` frame 54:15 (400 x 772) and
`LoginForm` frame 58:3. The Digital ID button, its OR divider, exclusive runtime
asset registration and exclusive styles were removed because that option is not
present in the inspected frame. The original fingerprint PNG/SVG remain unchanged
as provenance assets and are not bundled through Login.

The original 941 x 1672 GovAI logo file and its documented 154.17% x 271.32%
crop at -30.13% left / -63.6% top remain unchanged. Removing the unused section
and using section-specific form spacing lets the existing responsive height
budget reach the full Figma 196 x 198 slot at the 400 x 772 reference viewport
when safe-area-adjusted space permits. Short windows retain the proportional
112-dp floor and natural scrolling; text is not scaled down. Fields, Back,
password visibility, Forgot Password and Sign up retain at least 48-dp targets.

Identifier/password validation, password reveal, disconnected notices, connected
development-demo Login -> Chat Home, all Welcome/onboarding origins, keyboard-first
Android Back, cleanup and production guards are unchanged. The visible Back action,
honest disconnected/demo notice and accessible target sizes remain intentional
adaptations where they differ from the static Figma frame.

A local-only regression check rejects Digital ID/OR runtime UI, checks two
retained inputs and 48-dp controls, and synthetically verifies the
196 x 198 reference logo slot, unchanged crop geometry and proportional short-
window floor. These measurements use mocked React Native layout and are not
physical Android rendering.

Automated validation on November 4 passed TypeScript, Expo dependency
compatibility, connected-demo, Forgot Password, Register and Chat Home checks.
Android/Hermes production exports were built with the connected-demo flag both
off and on; both produced the same `index-f90f64ad9c8c67b0b4829737cb4d67db.hbc`
bundle name. Production-exclusion checks passed against both outputs, including
absence of Digital ID copy and the retained fingerprint PNG. No emulator or
physical Android rendering, keyboard, font-scale or TalkBack test was performed.

## Greeting and Login demo - October 28, 2026

This section supersedes older statements below that Login never opens Chat Home
or that the greeting contains a fixed sample name. Normal authentication remains
disconnected. The eventual production destination requires backend-confirmed Login;
no production authentication route, persistent session, credential storage or API
was added here.

pages/ChatHomePreviewScreen.tsx now contains separately guarded standalone/demo wrappers.
The shared presentation is components/ChatHomeContent.tsx. Its optional
`authenticatedDisplayName` is reserved for a future backend-authenticated adapter.
Without a nonblank trusted name, both visual/accessibility greeting text is "Hello".
A supplied name is trimmed and displayed as text, with wrapping. Current wrappers
supply no name; no identifier/email/registration-derived identity is passed.
Original assets, crop, styles, disabled actions and local-draft handling remain.

Connected mode requires both __DEV__ and exact EXPO_PUBLIC_CONNECTED_UI_DEMO=1.
Local required identifier/password checks run first, then both native inputs and
state, password visibility, errors and keyboard focus are cleared before navigating.
No credentials are passed to App or Chat Home. Submission/unmount latches plus
Login-visit guards reject repeats and callbacks from earlier Login/Chat visits.
Sign up, Forgot Password, explicit Back and unmount preserve credential cleanup.

Login notice in demo: "UI demo only. Credentials are not verified; valid fields open
Chat Home." Chat Home notice: "UI demo - Credentials were not verified. AI and actions
are disconnected." (The UI uses a middle dot separator.) Normal Login retains its
original disconnected notice and does not navigate, even with flag 1 in production.

Back: demo Chat Home dismisses its keyboard first; next Back clears the draft and
returns to fresh Login. Login keeps its original Welcome/onboarding origin.
Standalone Chat Home still returns to Welcome. Registration/Account Created and
NIC recovery/Check Email flows are preserved. No visible Exit button was added.

### Android top status bar - pending

The phone still shows a black top strip without readable status icons. The cause
is unresolved: actual native visibility, appearance, safe-area insets and window
measurements were not obtained. ADB and the Wi-Fi inspector had no connected phone.
The experimental SystemBars helper/backing, explicit-visibility policy, plugin
entry and investigation-only checker/instructions have been removed. Login and
Chat Home again use their previous expo-status-bar dark-icon declaration. Existing
safe-area owners, layout and keyboard handling are unchanged. No fix is claimed;
further investigation is deferred. The active development server is preserved.

### Navigation checks on a phone

Use the active demo server, or launch a separate available port with
`npm.cmd run demo:connected-ui -- --port 8097`. With dummy inputs, blank fields
must stay on Login; nonblank local fields open Chat Home with Hello and the
unverified-credentials notice. Keyboard Back dismisses the keyboard; another Back
returns to cleared Login, whose Back retains its original Welcome/onboarding origin.
The standalone `npm.cmd run preview:chat-home:dev` still exits to Welcome. No visible
Exit button is added. Native status-bar remediation is outside this deferred work.

### Checks actually run

TypeScript, Expo dependency compatibility, the Chat Home checker and all seven
previous regression suites passed for the greeting/Login demo. Coverage includes trusted
name fallback, both direct mount guards, required fields, cleanup on all exits,
repeat/stale submit and stale Chat exit, all six Login origins, Chat Back, preview
precedence and preserved safe-area/layout behavior.
Android/Hermes production exports were built with every flag enabled and disabled;
all six production-exclusion suites passed against both outputs. Checks reject demo
routes/notices in production and exclude Chat Home assets/font. The two exports have
the same Metro source-bundle hash; Hermes embeds different temporary compilation
paths, so the final .hbc files are not byte-identical.

Both actual launch commands above started and returned Android development bundles
containing shared Chat Home and the unverified-credentials notice, with
no hardcoded sample greeting. Verification servers were stopped afterward.
Optional historical source audits now exempt the specifically approved pages/LoginScreen.tsx
changes, which have dedicated behavior checks. Other comparisons
remain active; cached snapshot bytes were not rewritten.

Those mock regressions and production-exclusion diagnostics remain optional
local-only evidence and are not commands available in a GitHub clone.


## Initial GA-22 Chat Home standalone preview - October 28, 2026

This section records the initial standalone implementation of Figma frame 77:4.
Current integrated Login-to-Chat behavior and the later conversation/menu work are
documented above. The standalone preview remains available independently.

Launch from app/mobile in Windows PowerShell:

```powershell
npm.cmd run preview:chat-home:dev
# Optional available port:
npm.cmd run preview:chat-home:dev -- --port 8096
```

The launcher sets EXPO_PUBLIC_CHAT_HOME_PREVIEW=1 only in its child environment,
disables the four existing standalone flags and connected-demo flag, and writes
no environment file. App and direct screen entry require __DEV__ and exact flag 1.
Existing standalone previews and the connected UI demo retain priority if flags
are manually combined. The standalone route is separate from the current guarded
Login-to-Chat demo route. The screen, original avatar/icons and Fraunces font are
excluded from production even with flags set.

The initial visible notice was "Development preview · Sample content. Authentication
and AI are disconnected; actions are unavailable." The current wrapper uses the
neutral greeting documented in the Greeting and Login demo section above.
Local draft typing works; nothing is sent, stored, authenticated or generated.
Menu, notifications, all three suggestions, attachment, microphone, Send, and all
four bottom tabs are accessibly disabled and have no press handlers. Home remains
visually selected. There is no visible Exit preview button. Android Back dismisses
the keyboard first; the next Back clears the draft and returns to Welcome.
Unmount also clears the native input. No backend, AI API, microphone or file picker
is called; no session, token, draft persistence, dependency or EAS configuration.

The header, notice, greeting, suggestions, composer and bottom navigation share
one naturally overflowing ScrollView. Flex growth centers the landing content and
places composer/navigation at the bottom when content fits. Short windows and
large text can scroll the whole content, keeping input/actions reachable without
clipping text. Dedicated safe-area padding owns each edge once, with a separate
inner measured viewport for keyboard overlap. Resized keyboards add zero extra
overlap; overlay keyboards add only the measured covered area and conditionally
scroll the focused composer. No automatic keyboard inset compensation is layered
on top. All controls have at least 48dp targets; navigation labels are 12px instead
of Figma's 9px. Text scaling/wrapping remains enabled. Wide content is bounded at
480dp. The original avatar crop and explicit image dimensions are documented in
[asset provenance](assets/figma/chat-home/README.md); assets and font are local.

Files: pages/ChatHomePreviewScreen.tsx; guarded entry in App.tsx; one package.json launch
script; scripts/preview-chat-home.cjs and rasterize-chat-home-icons.ps1;
assets/figma/chat-home and this documentation. Optional mock checks are local-only.
AGENTS.md's local-only context now names GA-22 and retains every safeguard; its
existing .git/info/exclude entry remains, and the file is not tracked.

Validation performed:

```powershell
npm.cmd run typecheck
npm.cmd run check:expo
npm.cmd run export:android
```

TypeScript, Expo dependency compatibility, the new checker and all seven existing
regression suites passed. Production Android/Hermes export and all six exclusion
checkers passed with all flags enabled. New checks cover eight mode/flag cases,
standalone/connected-demo precedence, local draft typing, keyboard-first Back,
exit/unmount cleanup, repeated Back, loading/error exits, twelve disabled controls,
asset hashes/geometry and launcher arguments/environment isolation. Synthetic
layout/keyboard checks cover 320x568, 360x640, 360x800, 412x915, 600x960 and the
400x772 reference at font scales 1, 2 and 3.

The actual launch command was run on port 8096. Metro reported running and its
Android development bundle returned successfully with ChatHomePreviewScreen,
Fraunces_700Bold and the disconnected notice present. That verification server
was stopped afterward. Existing dependencies were checked, not installed.

Optional local tooling generated HTML approximations from the actual component
tree under .cache/chat-home-review.
Browser renders inspected at 400x772, 320x568, 600x960 and enlarged text verify
original asset placement/crop, readable wrapping and accessible overflow direction.
These renders translate mocked React Native primitives to HTML: they are not native
Android screenshots or evidence of TalkBack/IME correctness. Physical-device and
emulator testing were not performed. Remaining checks: Expo Go on a physical Android
phone, TalkBack labels/disabled states, font metrics, safe areas, native image crop,
keyboard resize/overlay, Back/focus behavior, scroll reachability and reference
comparison at default/enlarged text. No APK was installed.


## Connected development-only UI demo - October 27, 2026

From app/mobile in Windows PowerShell, launch the connected flow with:

```powershell
npm.cmd run demo:connected-ui
```

Optional free port: `npm.cmd run demo:connected-ui -- --port 8095`.
The launcher enables EXPO_PUBLIC_CONNECTED_UI_DEMO=1 only for its child process
and disables all four standalone preview flags there. It writes no environment
file. The app requires both __DEV__ and the exact dedicated flag value '1'.
Without both, Register and Forgot Password retain their disconnected notices.
Production ignores the flag. Login submission is unchanged and disconnected.
The normal Welcome/onboarding entry remains, so every Login origin can be tested.

Connected routes:

- Login -> Sign up -> Register. Existing required-field and raw password-matching
  validation runs first. Valid entries are cleared before Account Created opens.
- Account Created visibly says "UI demo — no account was created". Continue to
  GovAI opens a fresh Login. Exit preview is hidden in this mode. Android Back
  also opens fresh Login, never the submitted Register form.
- Login -> Forgot Password. Existing nonblank NIC validation runs first. Valid
  input is cleared before Check Email opens.
- Check Email visibly says "Preview — no email was sent" and always displays
  sample@example.test. Back, Use a different NIC, and Android Back open a fresh,
  empty Forgot Password form. No email is derived from the NIC.
- Forgot Password Back/Log in return to fresh Login. Login retains its original
  Welcome/onboarding return destination through both flows.
- Android Back dismisses an open keyboard before navigating. Local form values,
  errors, password visibility, and agreement state are discarded on leaving.
  A submission latch and route-aware callbacks ignore repeated/stale submissions.

No accounts/passwords are stored, no requests are sent, no tokens are generated,
and no authentication or email delivery is performed. Resend and Open email app
remain unavailable. There is no Home, OTP, or connection to Reset Password or
Password Updated. Sample account text remains static design content.

The two existing confirmation designs now have one shared presentation each in
components/AccountCreatedContent.tsx and components/CheckEmailContent.tsx.
Standalone preview wrappers keep their original flags, labels, disabled actions,
and exit destinations. Their launch commands are unchanged; standalone previews
retain priority if flags are combined manually. Presentation entry points also
check development mode and the appropriate flag. Assets, styles, image crops,
shared layout helpers, English-only behavior, and form keyboard helpers remain.
The approved demo changes only confirmation labels, hides Account Created Exit,
and enables its Continue action with a caption identifying Login as its destination.

Shared repeat checks:

```powershell
npm.cmd run typecheck
npm.cmd run check:expo
npm.cmd run export:android
```

The Register harness defines __DEV__ explicitly. Its direct command runs the
unchanged form/navigation assertions in separate production and development
processes with all demo/preview flags disabled. The connected-demo checker also
covers both modes with the dedicated flag absent, 0, 1 and true.

The Reset Password and Password Updated checkers retain their optional task-time
snapshot assertions. On October 28, 2026, exactly four reviewed expectations in
each existing local snapshot were updated for the approved connected-demo work:
pages/AccountCreatedPreviewScreen.tsx and pages/CheckEmailPreviewScreen.tsx now delegate to
shared guarded presentations; pages/RegisterScreen.tsx and pages/ForgotPasswordScreen.tsx
add validated development-demo callbacks and cleanup. No other snapshot values
were changed; existing App.tsx/package.json exemptions were not expanded.
Snapshots remain ignored local cache files and are excluded from commits.
These two files are optional local audit evidence, not required test fixtures.
They compare local source bytes with a particular implementation-time baseline;
they are not the expected results for form behavior, navigation, guards, assets,
or production exclusion. No developer needs to obtain or recreate them. If an
audit file exists, all its existing assertions run and a mismatch still fails.
If absent, the checker prints an explicit INFO message that this historical audit
is unavailable. Required assertions execute independently in either case.
Portability verified on October 28, 2026: both files were physically moved aside,
then TypeScript, Expo compatibility, all seven direct checkers, and all five
export-aware checkers against each fresh flag-off/flag-on export passed. No
filesystem mocks or assertion overrides were used. Both local audit files were
restored byte-for-byte afterward. This simulates missing local evidence in the
existing installed checkout; it is not a separate clean dependency installation.

A fresh checkout needs only the tracked source/assets, package.json and lockfile;
run `npm.cmd ci` to install dependencies, then use the shared npm commands above.
Optional mock regression and historical audit scripts are not distributed.

Local historical results below are retained for context, but their checker and
snapshot commands are not available in a GitHub clone.

Validation on October 28, 2026: all seven direct checkers, TypeScript and Expo
compatibility passed without skips. The Register command passed in both modes.
New connected checks cover 8 mode/flag combinations, real form handlers, invalid
submissions, raw matching, cleanup, duplicate submissions, all 6 Login origins,
keyboard-first Back, both confirmation destinations, fresh recovery forms,
standalone priority, 36 synthetic confirmation layouts, and launcher arguments.
Mocks are not native rendering or accessibility verification.

Production-exclusion validation was previously run locally against fresh
Android/Hermes flag-off and flag-on exports. The optional diagnostic commands are
not included in the repository.

All five export-aware checkers passed against both fresh outputs with snapshots
active. Android/Hermes production exports were checked with the dedicated demo flag
absent and set to 1. Exclusion checks cover demo screen names, unique notices,
sample email/name, and both exclusive confirmation images. Exports are not APKs.
The actual documented demo command was started with --port 8095: Metro status and
an Android development bundle returned HTTP 200; the bundle contained both demo
screens and notices. Only that verification server was stopped. Hermes execution
required sandbox execution permission. No dependency installation or EAS setup.

Pending physical Android checks: scan the QR in compatible Expo Go; compare the
400x772 reference; test narrow/short/tall/wide windows and default/enlarged text;
inspect both confirmation labels and hidden Exit, image crops, safe areas,
scrolling, TalkBack, keyboard Next/Done and Back, both complete flows, cleared
re-entry, and Login Back to each original destination. No phone/emulator rendering
was performed during this change.

Earlier sections below are historical implementation notes; the current connected
demo behavior above supersedes earlier statements about unavailable transitions.


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
visibility toggle. The original version rendered Digital ID, Forgot Password and
Sign up as unavailable. Later approved work connected the local navigation links,
and GA-55 removed Digital ID to match Figma 54:15. No social authentication,
dependencies or backend changes were introduced.

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
artwork/form with the superseding Figma 54:15 alignment above. Previous physical-device checks remain pending;
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

Validation: TypeScript and optional local mock regression checks passed. The local tests covered required fields, arbitrary identifiers without invented format rejection, Unicode names, raw/mismatched passwords, both visibility toggles, optional agreement, disabled Terms, notices, explicit exit and detached-ref unmount clearing; 18 synthetic layout/convergence cases and 36 resize/overlay keyboard cases across 320x568, 360x640, 360x800, 412x915, 600x960 and reference 400x772 with font scales 1/2/3; all six Login origins and Register Back routes; 12 original asset hashes. Existing file hashes and exact Login-only wiring changes were checked. PNG icons were visually inspected as assets, not rendered app UI. The optional mock checker is not included in Git.

Android production export passed with Hermes and 43 bundled assets. It emitted only the environment warning that FORCE_COLOR overrides NO_COLOR. No emulator or physical-device rendering was performed.

Pending Android phone checks: compare with Figma at reference size; inspect compact header/card, natural overflow, safe areas/system bars, field/placeholder wrapping, Unicode-name shaping and TalkBack focus/disabled Terms/checkbox announcements. Test the five Android sizes with normal/enlarged text, cutouts and both navigation modes. Exercise all six fields and keyboard Next/Done, first-invalid focus, errors and mismatch reveal, both password toggles, keyboard open/close/resize/overlay, action reachability, every exit and fresh re-entry. Confirm unchecked agreement permits local validation and there is no network/account/verification action.

## Account Created development preview

Run `npm.cmd run preview:account-created:dev` to start Expo Go with a process-local preview flag. Both __DEV__ and EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW=1 are required by App and the screen. Normal startup has no preview entry. Production ignores the flag; the production Android export inspected with flag=1 excludes the preview module and its icon. No APK build is involved.

All content is explicitly labelled as a sample: no account has been created. Rathishan is static Figma copy, not user data. Continue is disabled because its destination is unimplemented. Exit preview and Android Back return to Welcome. Register validation, submission, navigation and credential cleanup remain unchanged. The preview sends no requests and persists no data.

Shared validation commands are `npm.cmd run typecheck`, `npm.cmd run check:expo`
and `npm.cmd run export:android`. Optional guard/navigation/layout regressions were
run locally and are not included in Git.

Guard/navigation and layout checks use mocked React/native measurements, not native rendering. Synthetic windows: 320x568, 360x640, 360x800, 412x915, 600x960, and reference 400x772; font scales 1, 2 and 3. Repeated intrinsic measurements converge in these mocks.

Pending Android phone checks: actual wrapping, measured-layout stability, default/enlarged text, safe areas, artwork crops, scrolling/footer reachability, TalkBack reading and disabled-control announcements, Exit/Back, and Login/Register keyboard and credential-cleanup regression. No emulator or physical-device verification is claimed.




## Forgot Password (UI-only)

Login's Forgot Password link opens the frame 399:2071 screen. It explicitly clears Login identifier/password/visibility/error state before leaving. Back and Log in return to a fresh Login while retaining the original Welcome/onboarding return destination. Android Back dismisses the keyboard first, then returns to Login; exit/unmount clears the NIC and validation state.

The NIC starts empty and is checked only for nonblank input. No format, length, checksum or account-existence rule is inferred from the Figma example. Description: Enter your NIC number. Password recovery is not connected in this UI-only preview. Support note: Recovery support details are not available in this UI-only preview. Nonblank submission stays on this screen and says: Password recovery is not connected. No request was sent and no password was changed. No requests, reset tokens/OTPs, storage, logging, fake progress or recovery destinations are implemented.

Three new original icons are preserved under assets/figma/forgot-password with provenance; verified shared backgrounds/logo/Back are reused. Fonts, responsive text, hidden-indicator overflow, safe-area ownership and 48-dp targets follow existing conventions. Decorative space reduces first; keyboard resizing does not replace the closed-screen baseline.

Validation: shared TypeScript and Android export commands passed; optional local Forgot Password, Register/navigation and Account Created mock regressions also passed. Those local scripts are excluded from Git. No EAS configuration or native APK build is involved. Synthetic measurements are not device rendering. Pending phone checks: actual wrapping/crops, enlarged text, keyboard resize/overlay/focus/error reachability, Back/Log in cleanup, TalkBack announcements and safe/system bars at narrow/short/tall/wide windows.


## Check Email development-only preview

Run npm.cmd run preview:check-email:dev. Optional port selection: npm.cmd run preview:check-email:dev -- --port 8082. The launcher uses supported Expo start --go --lan arguments and default development mode. It sets EXPO_PUBLIC_CHECK_EMAIL_PREVIEW=1 and Account Created preview=0 only in its child process, preserves the parent environment, and forwards the exit status. No env file is written. An existing Metro server must not be stopped without authorization; choose a free port instead.

The App entry and direct screen mount require __DEV__ plus the Check Email flag. Production ignores that flag and excludes the dynamically required module and its exclusive mail icon. Account Created keeps its own separate guards; if both flags are set manually, Account Created takes precedence and exiting it does not open Check Email. No link is added to Forgot Password submission.

The compact notice reads Preview - No email sent. The footer keeps Use a different NIC with no visible Exit preview control; Back has an in-bounds 48-dp target. The intrinsic card uses reduced padding/gaps, and the burgundy disabled email button retains its unavailable caption. The address is always sample@example.test. No reset link exists or expires, Open email app and Resend are visibly/accessibly disabled with no handlers, and no network, mailbox, timer, token or resend logic is implemented. Back, Android Back and Use a different NIC close the preview and open fresh Forgot Password; The static resend row reads Didn't get it? Resend, with an adjacent unavailable caption and accessibility disabled state. The normal Forgot Password Log in flow preserves Login origin; standalone preview begins with Welcome as its original destination.

Run `npm.cmd run typecheck`, `npm.cmd run check:expo` and
`npm.cmd run export:android`. Optional local preview/recovery regressions previously
covered flag combinations, direct mount, disabled actions, routes, launcher
port/environment/status and synthetic logical-window/font-scale cases; they are
excluded from Git and do not prove native rendering.

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

Shared repeat checks from app/mobile:

    npm.cmd run typecheck
    npm.cmd run check:expo
    npm.cmd run export:android

For the flag-on production check, use a dedicated PowerShell shell or preserve
and restore any existing variable:

    $env:EXPO_PUBLIC_RESET_PASSWORD_PREVIEW = '1'
    node scripts/expo.cjs export --platform android --output-dir dist/reset-preview-flag-on
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

Shared repeat checks:

    npm.cmd run typecheck
    npm.cmd run check:expo
    npm.cmd run export:android

For a flag-on production check, use a dedicated PowerShell shell or preserve
and restore the previous variable value:

    $env:EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW = '1'
    node scripts/expo.cjs export --platform android --output-dir dist/password-updated-flag-on
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
