# Conversation assets

Source: [GovAI chat home, frame 97:26](https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/GovAI?node-id=97-26).
Design context inspected October 28, 2026. Original avatar PNG and vector SVGs
were downloaded from that context's original Figma asset URLs, never used remotely
at runtime. sha256.json records each original and raster hash.

Back and New chat have 24px SVG roots, displayed in 20dp slots; status 7dp,
ID card 22dp, service arrow 16dp, globe 14dp, and the Profile-slot asset 22dp.
PNG icons are transparent 3x rasterizations with the existing Chrome/System.Drawing
method in scripts/rasterize-chat-home-menu-icons.ps1, substituting this directory,
a separate .cache/chat-conversation-icons scratch directory and these icon names.
No SVG paths were edited. Reproduction must preserve each root's original aspect
ratio and crop the transparent capture to its root dimensions times three.

The original avatar is SHA-256 identical to ../chat-home/avatar.png. Conversation
node 679:2416 uses its own 32dp circular crop: image 99.74% width/height, left 1.25%,
top 0.75%. Landing retains its previously verified 62dp crop. Runtime uses the
conversation's original avatar file at that exact inspected crop geometry.

plus, mic, send, home, services and requests raster PNGs are byte-identical to
existing landing images and those existing assets are reused. Some SVG XML differs
while raster output matches. Originals/raster comparisons remain here for audit.
The Profile-slot asset follows this frame's actual export (named bell in Figma).
No copied iPhone status icons, camera island or home indicator are implemented.

The reference's fees, duration, steps and "checked Sep 2026" are unverified design
copy. Runtime substitutes approved mock labels and unverified service details.

The NIC sample includes a clickable official homepage reference using the existing
globe asset. Its visible label is exactly: "Official website · Demo reference, not verification of this response".
OfficialSourceReference.tsx hands only the approved constant `https://drp.gov.lk/`
to React Native Linking for Android's external handler/browser or chooser. No chat
text, credentials, personal data, query parameters or fragments are appended.
The homepage does not verify the mock response, sample steps, fees or timeframes.

The link uses accessible link semantics, a department-specific label, an external
browser hint and a minimum 48x48dp target. Duplicate taps are blocked while opening.
Handoff failure shows the accessible message "Couldn’t open the official website. Please try again."
and permits retry; late completion after unmount cannot update the reference.
Conversation and draft state are retained. Actual Android browser handoff, return
navigation, failure behavior and TalkBack remain device-testing gaps. Other service
actions remain disabled; mock/unverified notices and production guards are unchanged.
