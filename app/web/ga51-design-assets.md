# GA-51 shell design and asset record

Inspected and implemented 26 October 2026. Web frontend only.

Approved references: [03 - chat home v2](https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/?node-id=508-269) and [04 - recent chats v2](https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/?node-id=523-2430). Right-aligned account controls approved by the user across both previews.

## Figma exports

Unmodified exports retrieved using curl from design-context asset URLs. All exports below are used; no temporary Figma URLs are used at runtime. Existing login exports remain unchanged. Active/inactive clock exports are distinct. The browser suite checks every shell export is loaded and its rendered width/height matches its root dimensions (emblem uses the supplied 57x48 slot).

| Local file | Figma export identifier | Slot | SHA-256 |
| --- | --- | --- | --- |
| emblem.png | fb6e1.png | 57x48 | 65121dd980afd15145ce93b30272268898268443d0071525e0ceb7c18b947bc4 |
| globe.svg | 7e812.svg | 18x18 | 27b68ba7c539cdf1049b1d315d13c2463281a22d46001d7f889c12c9826a51e8 |
| chevron.svg | 9840c.svg | 16x16 | c8c5c8f9fe03152d8b1f2c40894d44b78cbd727cdeaf03bc8961e470daa04434 |
| notifications.svg | 88a4a.svg | 40x40 | 4d3e4f7e67587904e96e3c12887ce5ec18ed4eec57461a9d783e2285d61d52fc |
| user-chevron.svg | c3d78.svg | 18x18 | b921c7cd67d95c0de791bf77cf4c000218cf610770a8f9e2090ac552759a7371 |
| new-chat.svg | ef00b.svg | 18x18 | 23dfb3ebeee72dffb24c25fd0c2c8e0ad761e008e33814d169bba546d1f141b6 |
| recent.svg | ebd39.svg | 20x20 | 01900eafec45c09f7773e9d0a11b98e0f1b31b796a362e8d11751b923c279df0 |
| recent-active.svg | 32d17.svg | 20x20 | eea2d753cf9d6dc9f6c1925899c9129973c19e2688ca7f98e40cd819c4576b78 |
| requests.svg | ede3b.svg | 20x20 | ea77f2ee2d0a860dfa19d87754ebd5eff4f1d51904978ef51e7e89b2cc2a25eb |
| services.svg | 909a3.svg | 20x20 | 475571ff8f16e8e8a56a19b2e6d56a567f23b1ac5b18e72f369110122a91e95e |
| message.svg | a3289.svg | 16x16 | 95e513adafeb70856845e9c07a1e564411e523f8fbbf50fe2fe569cd3ee6030b |
| help.svg | 87dca.svg | 20x20 | 72076620e3022a60b298d3a39f97b24e71943bd919fd71d5b569095058852176 |
| settings.svg | e690b.svg | 20x20 | 2672ce5c82ee595d572d2cec023a53daf2c8ffe9d1885e067a08709170be9830 |
| crown.svg | 6eb81.svg | 16x16 | 0ef028c7037e4dbb10121f6ce428eb3fd768f1201e4a220b83cf4d1a5b17c1f0 |
| plus.svg | 910b4.svg | 22x22 | 918e6027912f7f38cc751b7d18d8325390284c59aab7f9cc3c2f7df8bd4e4475 |
| mic.svg | 0150f.svg | 22x22 | e1c9b813622dcfcc9bb6f1958061f471b4573a94a957adbdcf507e47ea265b4e |
| send.svg | 81313.svg | 22x22 | ef3cf86f54d611e2e84faa7985aca72a17926d375928c882cac5077852891304 |
| suggest-search.svg | 9edeb.svg | 20x20 | 87431260f59d8d62abe83070d63b1862c6ae783d4b6e46fdaa52b3e1483e2469 |
| suggest-status.svg | 2fef0.svg | 20x20 | ed23d3e6cf0ac2213bf769a0a427194c45b8a712605256fd2fe556881c5cb28d |
| suggest-request.svg | 2c981.svg | 20x20 | 4f026748083e273a46d72d5c2c38f37eb413a62ce42088d9fdc8222e91c068a1 |
| suggest-documents.svg | eef97.svg | 20x20 | 139d0e02b37b552b8cd97dca78be06ea03fb148cc13edda82e785bcdeb19b933 |

## Official fonts and licences

Unmodified variable TTFs downloaded from the official Google Fonts repository, with each family's complete copyright and SIL Open Font Licence 1.1 file distributed beside it. OFL permits embedding and redistribution with the licence; fonts are not sold separately or modified. Local filenames are shortened; font binaries retain their original internal names.

- [Inter official binary](https://raw.githubusercontent.com/google/fonts/main/ofl/inter/Inter%5Bopsz%2Cwght%5D.ttf); [licence](https://github.com/google/fonts/blob/main/ofl/inter/OFL.txt). Local: public/assets/fonts/Inter.ttf and inter-OFL.txt; SHA-256 29160a80ff49ddcab2c97711247e08b1fab27a484a329ce8b813d820dc559031.
- [Montserrat official binary](https://raw.githubusercontent.com/google/fonts/main/ofl/montserrat/Montserrat%5Bwght%5D.ttf); [licence](https://github.com/google/fonts/blob/main/ofl/montserrat/OFL.txt). Local: public/assets/fonts/Montserrat.ttf and montserrat-OFL.txt; SHA-256 0f7b311b2f3279e4eef9b2f968bcdbab6e28f4daeb1f049f4f278a902bcd82f7.
- [Fraunces official binary](https://raw.githubusercontent.com/google/fonts/main/ofl/fraunces/Fraunces%5BSOFT%2CWONK%2Copsz%2Cwght%5D.ttf); [licence](https://github.com/google/fonts/blob/main/ofl/fraunces/OFL.txt). Local: public/assets/fonts/Fraunces.ttf and fraunces-OFL.txt; SHA-256 177ff6c0f14e5550a3c624247cd1189611d4eb65d000b14944c63d967958abbb.

Shell CSS uses GovAI Inter/Montserrat/Fraunces family aliases, weights from Figma, and Fraunces SOFT 0 / WONK 1. Edge platform-font inspection confirms actual custom-font glyph rendering, not merely CSS fallback declarations. The precise font versions used by the Figma author were not supplied, so a byte-identical typography match is not claimed. The existing login preview intentionally retains its system sans-serif/Georgia fallbacks to preserve its current appearance.

## Deliberate preview differences

Synthetic Harshan profile, sample plan labels, static Recent examples, unavailable-action labels and login-preview link distinguish this layout from an authenticated product. Home composer is disabled and suggestions are static. Recent content is an explicit not-implemented panel; no history/search feature is implemented. Its shared header/sidebar/active state follow the reference. Composer placeholder uses #666 for readable preview copy rather than the original low-contrast placeholder grey.

At <=900px the website uses a modal navigation menu and responsive content wrapping. This is a proposed website adaptation, not an approved mobile design and not work on app/mobile. Only Windows Edge has been browser-tested; no screen-reader or physical-device claim.
