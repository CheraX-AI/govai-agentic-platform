# Login artwork provenance

Figma: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=54-14

Original downloaded files retained unchanged. All four PNGs SHA-256 match the existing original loading artwork: top-ribbon.png (1205 x 1305), skyline.png (1193 x 1319), bottom-ribbon.png (1306 x 1205), logo.png (941 x 1672). Login-specific display geometry is kept separate: top ribbon cover 272 x 296, skyline contain 723 x 451 with 45% opacity, bottom ribbon cover 237 x 227. Logo uses the documented nested 154.17% width, 271.32% height, -30.13% left, -63.6% top crop inside 196 x 198. Original proportions preserved within the crop rounding. Only decorative geometry scales with width; native form text does not shrink.

Original SVGs user/lock/eye/fingerprint retain Figma paths and colors. Transparent PNGs are 72 x 72 rasterizations from headless Chrome/System.Drawing using the existing approach, displayed in explicit 18 x 18 field slots and 22 x 22 fingerprint slot. Icons visually inspected. No packages installed and no temporary asset URLs used at runtime.

Native Android owns system chrome; the mock device, time, Dynamic Island and home indicator are excluded. Added visible Back, sign-in-unavailable notice, required-field errors and >=48dp controls are approved UI/accessibility adaptations. Large text can push artwork/form apart and scroll.
