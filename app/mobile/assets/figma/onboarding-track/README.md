# Track assets and geometry

Figma source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=284-799

Track independently specifies: display 400 x 772, hero 400 x 382, panel start y=340, overlap 42; image width 100%, height 169.78%, top -5.09%. Mapping uses the established 30-unit reference top/bottom bands and actual native safe areas. Natural body/footer measurements exclude flexible panel space. Responsive cropping preserves the complete logo, full-width artwork, proportions and overflow scrolling.

Track hero asset 33ac1.png and back/next SVGs a7580.svg / d06bd.svg exactly match existing files by SHA-256. The original hero in onboarding-discover and arrows in onboarding-meet are reused. No temporary URLs at runtime.

Original card SVGs retained unchanged: identity 02676.svg (24), calendar e15a3.svg (12), clock 9dda7.svg (14), chevron 8d164.svg (16), complete 3dbc7.svg (8), current 795d4.svg (18 including halo), pending 16915.svg (8). PNGs are transparent 72 x 72 rasterizations using headless Chrome and System.Drawing, with explicit native slots matching the listed design units. No packages installed. All seven rasterized assets visually inspected.

The card is native informational text and Views, not an image screenshot or interactive tracking UI. Completed/pending/current marker assets retain their design statuses. On narrow/large-text layouts the five stages reflow vertically in order; reference-width normal-text layouts retain the horizontal tracker. The fourth onboarding dot is active by user approval (Figma highlights the third).
