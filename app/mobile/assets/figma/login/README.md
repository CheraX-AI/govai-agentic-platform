# Login artwork provenance

Figma: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI-%E2%80%94-Mobile-App-Design?node-id=54-15

GA-55 was aligned against the live `Display` frame 54:15 (400 x 772) and
`LoginForm` frame 58:3. The frame has no Digital ID option or OR divider, so
neither is registered or rendered by Login. The original fingerprint source
files remain here as unchanged provenance assets and are not runtime inputs.

Original downloaded files retained unchanged. All four PNGs SHA-256 match the existing original loading artwork: top-ribbon.png (1205 x 1305), skyline.png (1193 x 1319), bottom-ribbon.png (1306 x 1205), logo.png (941 x 1672). Login-specific display geometry is kept separate: top ribbon cover 272 x 296, skyline contain 723 x 451 with 45% opacity, bottom ribbon cover 237 x 227. Logo uses the documented nested 154.17% width, 271.32% height, -30.13% left, -63.6% top crop inside 196 x 198. Original proportions preserved within the crop rounding. Only decorative geometry scales with width; native form text does not shrink.

Original SVGs user/lock/eye/fingerprint retain Figma paths and colors. Transparent PNGs are 72 x 72 rasterizations from headless Chrome/System.Drawing using the existing approach. Login displays user, lock and eye in explicit 18 x 18 slots; fingerprint.svg and fingerprint.png are retained but unused. Icons were visually inspected. No packages installed and no temporary asset URLs are used at runtime.

Native Android owns system chrome; the mock device, time, Dynamic Island and home indicator are excluded. Added visible Back, sign-in-unavailable notice, required-field errors and >=48dp controls are approved UI/accessibility adaptations. At the 400 x 772 reference viewport the existing responsive budget can reach the full 196 x 198 logo slot when measured content permits; shorter windows retain the proportional responsive floor and scrolling fallback. Large text can push artwork/form apart and scroll.
