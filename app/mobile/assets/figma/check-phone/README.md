# Check Phone reference adaptation

Reference: the Check Phone image supplied in this conversation, approved
November 4, 2026. It guides the card, badge, logo proportions, typography,
six visual code slots and footer actions; it is not a backend specification.
Phone border, camera island, simulated status icons and black home indicator
are excluded. Native safe areas and existing status-bar handling are preserved.

No image files were generated, copied or modified. Reused original assets:

| Purpose | Existing source | Adaptation |
| --- | --- | --- |
| Logo, ribbons, skyline | assets/figma/login/ | Original aspect ratios and verified logo crop |
| Back / white action arrow | assets/figma/register/back.png, next.png | Original images |
| Badge handset | assets/figma/register/phone.png | Green tintColor, 26dp |
| Badge message | assets/figma/chat-home/message.png | Green tintColor, 16dp, composed over handset |

Upstream provenance remains in the Login, Register and Chat Home asset READMEs.
The original Chat Home `message.svg` and unchanged PNG raster have SHA-256 values
`75260a55924d57c8a8b1558d83d9dbd66962295e263db1820c06e514c307a662` and
`2b4e76833dff8c558305cddf28b5f259c3359142fcfaa1121ac17bc4e5f324c8`.
The exact smartphone/check composite in the reference is not an available asset;
the approved composition reuses the original handset and message shapes. The
badge is 48dp with #edf6f1 background and #007a53 icon tint. The original opaque
logo background is unchanged. Its width-safe 150 × 151.531dp slot at 400dp uses
the established crop: 154.17% width, 271.32% height, left −30.13%, top −63.6%.

The card is intrinsic, with 16dp horizontal / 8dp vertical padding and 4dp section
gaps. Decorative spacing was reduced to fit normal state without reducing fonts
or interactive targets. All interactive controls have at least 48dp targets;
the six digit slots are one accessible native input, not six separate controls.
Text scaling, hidden-indicator overflow and focused-field/error reachability
remain enabled. The composed icon is decorative and hidden from accessibility.

The masked number is fixed sample copy, never derived from the NIC. The no-SMS
notice and disconnected verification caption prevent delivery/authentication
claims. The local six-digit format is unconfirmed by the backend. Resend is
disabled; no delivery, expiry, retry rule or verification service is implemented.

Corrected HTML simulations passed 25 cases, including six viewport sizes at
text scales 1/2/3. Default card height at 400 × 772 is 497px in a 692px content
viewport after simulated safe-area padding, with normal state fitting. Short
windows, larger text and error/notice growth retain scrolling. This does not
establish native Android rendering, IME behavior, real paste/selection or TalkBack
acceptance; those require device testing.
