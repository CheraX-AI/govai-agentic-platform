# Citizen profile photos

GA-254 uses the existing SeaweedFS S3 service. No new database or registration
architecture is introduced. Only the server-generated object key is stored in
`auth.citizen_details`; images and storage credentials never enter PostgreSQL.

## Configure

Install/check GA-252 migration 007 and enable GA-253 first. In the auth-service's
private environment, set `PROFILE_PHOTO_ENABLED=true` and configure:

```text
PROFILE_PHOTO_S3_ENDPOINT=http://127.0.0.1:8333
PROFILE_PHOTO_S3_BUCKET=govai-citizen-photos
PROFILE_PHOTO_S3_REGION=ap-south-1
PROFILE_PHOTO_S3_ACCESS_KEY=<server-only credential>
PROFILE_PHOTO_S3_SECRET_KEY=<server-only credential>
```

Provision a private bucket and appropriately scoped S3 credentials on existing
SeaweedFS before enabling uploads. The app does not create buckets or modify
SeaweedFS configuration. Do not reuse the public knowledge-document namespace
or make the photo bucket public. This is an operational dependency: the app
cannot enforce a storage administrator's bucket policy.

The feature defaults to false. HTTP is accepted only for literal loopback or
localhost development; remote endpoints require verified HTTPS. A containerized
auth-service must use an appropriate reachable HTTPS endpoint rather than its
own loopback. Credentials are read only by the server. The official AWS Go v2 S3
SDK signs requests, uses path-style addressing and one attempt per operation.
Storage errors returned to clients are fixed safe messages, without SDK errors,
keys, URLs or credentials. No SDK body/credential debug logging is enabled.

## HTTP contract

| Operation | Native/mobile JWT | Browser cookie/session |
| --- | --- | --- |
| GET | `/api/v1/profile/photo` | `/api/v1/web/profile/photo` |
| PUT | `/api/v1/profile/photo` | `/api/v1/web/profile/photo` |

These use the existing profile authentication adapters. Native requires HTTPS,
verified citizen JWT and ready profile. Browser requires its secure cookie and
`X-GovAI-Web: 1`; PUT additionally requires the exact approved Origin and
`X-CSRF-Token`. Query parameters/citizen IDs/object keys are not accepted.

Upload the raw image bytes (not multipart or JSON), with `Content-Type:
image/jpeg` or `image/png`. First GET the profile to obtain its revision. Send
`If-Match: "0"` for revision 0, with exactly one strong quoted numeric value.
Missing revision returns 428; malformed revision returns 400; stale revision
returns 409. Profile detail edits and photo replacements share one revision.

The maximum upload is **2 MiB (2,097,152 bytes)**; a valid image exactly at that
limit is accepted, including a JPEG with padding. A 10 MiB upload is rejected.
The actual image signature, decoder, format and dimensions are checked, not the
filename or MIME label alone. Maximum decoded area is 16 million pixels. Images
are re-encoded (JPEG quality 90 or PNG) to remove metadata and trailing data;
stored output is also limited to 2 MiB. JPEG is recompressed. Two simultaneous
decode/replacement operations are allowed per service instance; excess work
returns temporary unavailability. Existing server deadlines still apply.

Successful PUT returns:

```json
{"revision":1,"has_photo":true,"photo_url":"/api/v1/profile/photo"}
```

The web adapter returns its corresponding authenticated web path. GET profile
also supplies that path when photo support is enabled and a reference exists.
`photo_url` is not a public URL or a signed SeaweedFS URL. GET photo requires the
same authentication, returns image bytes, Content-Type and a quoted revision
ETag, with no-store/nosniff headers. No photo is 404; storage failure/missing file
for an existing reference is 503. Both clients must fetch images using their
normal authenticated mechanism; a native bearer-protected URL is not public.

Unsupported/forged/corrupt images return 415 `unsupported_photo_type`;
oversized files return 413 `photo_too_large`. Storage failure returns 503.
Authentication/session/origin errors follow GA-253. No error echoes a filename,
object key, credential, submitted image or another citizen's profile.

## Replacement consistency

1. Resolve the authenticated issuer/subject and load only that ready profile.
2. Validate revision and image, then generate a random 256-bit object name.
3. Upload the new object to the configured private bucket.
4. Persist its key using the atomic owner/revision predicate; increment revision.
5. Only after a confirmed save, attempt deletion of the old server-owned object.

Upload failure leaves the prior reference intact. A definite revision conflict
attempts candidate cleanup. An ambiguous database/commit result retains the
candidate because it may already be the current reference. A cleanup failure
does not roll back a valid saved reference. Consequently storage outages or
ambiguous outcomes can leave unreferenced objects. A durable cleanup/reconciliation
queue is not implemented here; operators should reconcile unreferenced objects
against PostgreSQL with a suitable retention window. Never bulk-delete objects
based solely on a failed client response. Existing unrecognized legacy keys are
not automatically deleted. PostgreSQL and SeaweedFS do not share a transaction.

## Verification

Unit tests cover exact size limit, invalid types, forged/corrupt content,
metadata stripping, ownership, revision conflicts, upload failure, ambiguous
commit and cleanup failure. HTTP tests cover both authentication adapters,
CSRF, the 2 MiB router limit, missing/stale revision, isolated photo access and
retrieval. PostgreSQL tests check reference persistence, ownership and CAS.
S3 adapter tests check signed requests and sanitized errors. Opt-in real SeaweedFS
integration uses `GOVAI_PHOTO_TEST_ENDPOINT` against disposable infrastructure
with synthetic credentials/bucket `govai-photo-test` only.

No live citizen data, private environment or development bucket is changed by
the automated tests. Web/mobile screen integration and device photo selection
remain GA-255/GA-288. Live deployment credential/bucket policy review and full
cross-device flows remain GA-256 integration checks.

Validation record: the full auth-service suite, disposable PostgreSQL tests and
`go vet ./...` passed for this change. Signed S3 requests were verified against
a local HTTP fixture. The real SeaweedFS integration test was not run: its Docker
image and official-binary downloads did not complete in this environment.
Run the opt-in storage test against disposable SeaweedFS before claiming that
external integration verified. No browser/device or live Keycloak test is claimed.
