# Chat Home asset provenance — GA-22

Source: [GovAI chat home2, frame 77:4](https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/?node-id=77-4), inspected October 28, 2026. Reference display: 400 × 772 inside a 428 × 800 device frame. This directory records the landing assets; the related drawer and local mock conversation are implemented separately.

Original images downloaded directly from the Figma design-context response. Temporary URLs are not runtime sources. `sha256.json` records the unchanged originals, native rasterizations, font and license; the preview checker verifies these hashes.

| Local original | Figma layer | Display geometry |
|---|---|---|
| avatar.png | 683:2451 | 62 × 62 circular slot; image 68.7208 × 64.5544, left −2.7156, top −3.1434 |
| menu.svg | 77:77 | 20 × 20 |
| notifications.svg | 88:6 | 20 × 20 |
| message.svg | 683:2458 / 2463 / 2468 | 16 × 16, repeated on the three suggestions |
| plus.svg | 674:2430 | 20 × 20 (original root 24 × 24) |
| mic.svg | 77:104 | 17 × 17 |
| send.svg | 97:24 | 17 × 17 |
| home.svg | 81:19 | 18 × 18 within the 36 × 28 highlight |
| services.svg | 84:4 | 22 × 22 |
| requests.svg | 86:4 | 16.5 × 16.5 within the original 22px document wrapper |
| profile.svg | 81:22 | 22 × 22; source layer misleadingly named “bell”, visible asset is a person |

Native `Image` callsites use the matching PNGs with explicit dimensions and `contain`. Originals retain their SVG root geometry. `scripts/rasterize-chat-home-icons.ps1` uses existing headless Chrome/System.Drawing tooling to render transparent native rasters at approximately 3× source root size; no dependency was installed. Fractions round to whole raster pixels.

The avatar is the original 250 × 247 image. The callsite deliberately reproduces Figma's explicit image box (110.84% width, 104.12% height, −4.38% left, −5.07% top) using `stretch` inside the circular clipping slot. This preserves the approved reference crop rather than replacing it with an independently chosen `cover` crop. Its separate width/height factors are intentional design evidence.

The 40px button discs remain visual elements inside 48px targets. Navigation labels increase from 9px to 12px and grey text/placeholders use #666 for readability. Text wraps/scales and containers grow; no system font scaling cap or single-line clipping is applied. The 26px Fraunces heading uses a 30px line height for readable native metrics.

System clock, signal, Wi-Fi, battery, device border, dynamic island, camera and home indicator are omitted: Android owns system chrome. The illustrated avatar is decorative sample content, not an authenticated user identity.

## Fraunces

`fonts/Fraunces-Bold.ttf` is a static normal-style weight-700, optical-size-9 build supplied by Google Fonts:

- [CSS specification](https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght@9,700)
- [Font source](https://fonts.gstatic.com/s/fraunces/v38/6NUh8FyLNQOQZAnv9bYEvDiIdE9Ea92uemAk_WBq8U_9v0c2Wa0K7iN7hzFUPJH58njr1603gg7S2nfgRYIcUByjDg.ttf)
- [Upstream license](https://raw.githubusercontent.com/google/fonts/main/ofl/fraunces/OFL.txt), retained verbatim in `fonts/OFL.txt`.

The font is loaded locally only inside the guarded Chat Home preview. No font package or runtime font download was introduced. Browser approximation review used the actual bundled font; native Android font rendering remains a phone check.

## Prototype evidence and current local interactions

Figma prototype destinations: Menu → 306:1388; notifications → 270:577; suggestions and Send → 97:26; Services → 114:2; My Requests → 230:76; Profile → 186:464. Attachment, voice and selected Home have no reactions in the inspected landing frame. Figma does not establish authentication, AI, recording or message-processing behavior.

In the explicit guarded development previews, Menu opens the existing drawer. All three suggestions open a local mock conversation with their selected prompt. The composer accepts a local draft; Send requires nonblank text and opens that conversation with the entered prompt. No chat is transmitted to an AI/backend. The greeting is Hello unless a trusted authenticated display name is supplied; current previews supply none.

Notifications, attachment, microphone, and the landing Home/Services/My Requests/Profile tabs remain accessibly disabled. Conversation Home is separately enabled to return to landing; other service destinations are not implemented. The drawer provides local New chat reset, Close/outside-tap/Back, and connected-demo-only Logout. Standalone Logout remains disabled. These interactions reuse the approved landing layout and assets; authentication remains disconnected and real authentication requires a backend.
