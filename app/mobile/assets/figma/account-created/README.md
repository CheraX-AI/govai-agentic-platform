# Account Created preview artwork

Source: Figma GovAI, node 399:2015, inspected October 26, 2026.
https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=399-2015

The original check-circle.svg is retained unchanged. Its SHA-256 is
`a7830f94f602c35cb617c12e2b7616c26200e15d772d32999bd6c6fdbd1cbb80`.
check-circle.png is a transparent 136 x 136 rasterization, visually inspected,
displayed at the frame's 34 x 34 logical size inside a 72 x 72 badge.
PNG SHA-256: `77ccb20145aa451865e07086c504a13a0a4b3e0a803a42257d7524e65a4a7255`.

Downloaded frame backgrounds and logo matched the existing Login files by
SHA-256; the forward arrow matched Register's next.svg. Those assets are reused
without alteration. No temporary Figma asset URL is used at runtime.

The frame's own logo slot is 150 x 151.531, with its original crop proportions.
The card grows with text; decorative header space reduces before content
overflows. Text scales normally and overflow scrolling has no visible indicator.
The sample notice, Exit preview control, and disabled-destination explanation
are approved preview additions. A large notice joins the scrolling content
instead of consuming the viewport. Continue has no action.

Automated checks cover guards, mocked measurements and navigation, not native
rendering. Phone checks for wrapping, safe areas, TalkBack, and enlarged text
remain pending. The production export was inspected with the flag set: the
preview-only icon and sample-screen strings were absent.
