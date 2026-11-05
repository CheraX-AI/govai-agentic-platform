# Forgot Password artwork

Figma frame 399:2071, inspected October 26, 2026:
https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=399-2071

The frame's four background/logo exports exactly match existing Login assets
by SHA-256. Its Back arrow exactly matches Register's back.svg/back.png.
Those originals are reused without alteration. Device chrome, time, camera,
Dynamic Island and home indicator belong to the operating system.

| Original export | Local source / raster | Native size | Raster size |
| --- | --- | --- | --- |
| 4bba6.svg | lock.svg / lock.png | 34 x 34 in 72 x 72 badge | 136 x 136 |
| 3ac66.svg | identity.svg / identity.png | 18 x 18 | 72 x 72 |
| 47c35.svg | info.svg / info.png | 16 x 16 | 64 x 64 |

All three SVGs are preserved unchanged; source hashes are checked by
an optional local regression script that is excluded from Git. PNGs are transparent 4x rasterizations using
the existing Chrome/System.Drawing method and were visually inspected. The
original lock/NIC color is #A30F2D; info color is #2D6FA6. No remote asset URL is
used at runtime and no new dependency is required.

The reference logo slot is 150 x 151.531 with the original nested crop:
154.17% width, 271.32% height, left -30.13%, top -63.6%. The fit correction
retains this slot at 400dp width; width-safe slots are 130 x 131.327 at 360dp
and 90 x 90.919 at 320dp. Only available width limits the logo: card growth and
keyboard changes no longer shrink it. Header/card separation is 8–12dp.
Card padding is 12dp vertically, the decorative badge 40dp and lock icon 24dp;
these do not change the underlying asset bytes. The spacing refinement uses
explicit section margins: badge/title 8dp, description/methods 10dp, methods/NIC
10dp, NIC/checkbox 8dp, caption/Continue 8dp and Continue/login target 6dp.
Related heading/description, label/control and checkbox/caption gaps are 4dp.
The card has no uniform gap or fixed height. All interactive
targets remain at least 48dp and text sizes remain unchanged. Card and vision heights are intrinsic
measurements with equality guards; the card's height never depends on header
height. Keyboard-driven viewport changes do not replace the closed baseline;
focused fields use current native measurements. Overflow scrolls naturally,
with the indicator hidden. Back and Log in targets are at least 48 dp.

Recovery-method adaptation approved November 4, 2026: the supplied raster
reference guides the card, centered lock badge/title, Email/Phone radio controls,
NIC field, simulated checkbox, red Continue action and Back to log in link.
Its generated branding and device chrome are not copied. All original assets
listed above, logo crop and existing keyboard/safe-area ownership are retained.
Radio circles and checkbox are native styled views; no assets or dependencies
were added. Readable text, 48-dp controls and overflow take precedence over
compressing the taller form to the reference's image dimensions.

Email is selected initially. NIC validation remains nonblank-only and the local
simulated checkbox is required. No real CAPTCHA, account lookup, request, SMS,
email, token or password change occurs. The fit correction removes the yellow
banner and sample destination; the Demo check tag and “Simulated check — no
security verification.” caption remain, with an explicit simulated/not-real-CAPTCHA
accessibility label. The exact subtitle is “Enter your NIC number and choose a
recovery method.” The guarded connected-development Email path opens the
existing Check Email demo. The approved Phone extension uses the same nonblank
NIC and simulated-checkbox validation to open Check Phone in connected-demo mode
only. Normal disconnected Phone submission keeps its unavailable/no-SMS notice.
Only the method is passed to navigation; no account lookup is performed.
The visible Continue label does not claim recovery. Exit resets input, method,
checkbox and errors. Older single-method task notes are superseded by this section.

Automated cases cover synthetic logical windows 320x568, 360x640, 360x800,
412x915, 600x960 and reference 400x772 with font scales 1/2/3. This is not proof
of native text/layout or IME rendering. Physical Android checks remain pending.
