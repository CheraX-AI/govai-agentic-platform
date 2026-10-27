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
scripts/check-forgot-password.cjs. PNGs are transparent 4x rasterizations using
the existing Chrome/System.Drawing method and were visually inspected. The
original lock/NIC color is #A30F2D; info color is #2D6FA6. No remote asset URL is
used at runtime and no new dependency is required.

The reference logo slot is 150 x 151.531 with the original nested crop:
154.17% width, 271.32% height, left -30.13%, top -63.6%. Reference header/card
separation is preserved when space permits; decorative spacing and logo size
reduce before readable text is affected. Card and vision heights are intrinsic
measurements with equality guards; the card's height never depends on header
height. Keyboard-driven viewport changes do not replace the closed baseline;
focused fields use current native measurements. Overflow scrolls naturally,
with the indicator hidden. Back and Log in targets are at least 48 dp.

Approved UI-only adaptations: empty NIC field; nonblank-only validation;
disconnected description and submission notice; neutral support note replacing
the unverified government instruction. The Send reset link control checks only
required input. No request, reset success, token, OTP, account lookup or password
change occurs. Inputs and errors are cleared on exit. The example NIC shown in
Figma is neither prefilled nor treated as a validation rule.

Automated cases cover synthetic logical windows 320x568, 360x640, 360x800,
412x915, 600x960 and reference 400x772 with font scales 1/2/3. This is not proof
of native text/layout or IME rendering. Physical Android checks remain pending.
