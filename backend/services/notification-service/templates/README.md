# Account notification templates (GA-271)

English drafts for the current account-event scope:

| Event | Editable wording |
| --- | --- |
| Successful login | en/login_succeeded.json |
| Password changed | en/password_changed.json |
| Email verified | en/email_verified.json |

Each JSON file contains separate email and in_app text. Body arrays keep source
lines short: email lines join with newlines (empty entries separate paragraphs),
and in-app lines join with spaces. Edit wording here, not in the event producers.
Email verification is a confirmation after verification, not an OTP or a link
that performs verification. Failed-login alerts and task reminders are outside
the current three-event contract.

renderer.go embeds these assets at build time. New requires the trusted HTTPS
issuer, validates the complete catalog, and creates an immutable renderer.
Render implements notifications.Renderer; RenderEmail supplies subject/body
without a recipient. Both validate the event and use fixed server-owned wording.
No event ID, account subject, issuer, password, token, IP address or dynamically
supplied link is inserted into the text. Rendering never sends email.

The renderer is ready for dependency wiring when the persistent processor and
delivery worker are integrated. It is not wired into the current health-only API;
processing_enabled remains false. Tests exercise the real processor with an
in-memory store and real MIME encoding without SMTP connections. These tests do
not establish PostgreSQL persistence, live queue processing or final delivery.

## Review and acceptance

These are English drafts awaiting human wording review. Confirm that the account
recovery guidance matches the final web/mobile flow before approval. Native-
speaker review has not been recorded. Sinhala/Tamil are excluded under the
agreed current English-only scope; if Jira still requires all three languages,
that scope needs updating or the translations and their reviews remain pending.
GA-271 must not be marked fully accepted on the basis of automated tests alone.

Run from the repository root after rebuilding the Docker checks image:

    docker run --rm govai-notification-checks:local go test -v ./templates

The SMTP MIME compatibility test lives in internal/delivery/email_templates_test.go
and runs as part of the normal internal/delivery tests. Neither command sends mail.
