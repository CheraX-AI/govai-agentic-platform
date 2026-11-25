# Citizen profile API

GA-253 extends GA-252 on the existing auth-service. The native and browser
adapters share the same profile service, request schema and response schema.
The browser adapter retains server-side token custody and CSRF protection.

## Enable explicitly

Install/check migration 007 using the existing privileged database tooling:

```powershell
go run ./tools/db migrate-profile
go run ./tools/db check-profile
```

Then set `PROFILE_API_ENABLED=true` in the existing auth-service environment and
restart the service. The flag defaults to false. At least one existing JWT or
web-session authentication path must already be configured, with HTTPS and a
database. When both are enabled they must use the same issuer. Startup and
readiness fail if the details migration or restricted runtime grants are missing.
The API never applies migrations automatically.

## Routes and authentication

| Method | Native/mobile | Browser |
| --- | --- | --- |
| GET | `/api/v1/profile` | `/api/v1/web/profile` |
| PUT | `/api/v1/profile` | `/api/v1/web/profile` |

Native requests require exactly one `Authorization: Bearer <access JWT>` over
direct HTTPS, the configured issuer/audience/client, verified email, citizen
role, and a ready linked profile. Cookies are not accepted as native identity;
a browser session cookie or web adapter header with a native request is rejected.

Browser requests use the existing `__Host-govai_session` HttpOnly/Secure cookie
and `X-GovAI-Web: 1`. PUT additionally requires the configured exact `Origin`
and one valid `X-CSRF-Token`, obtained through `/api/v1/web/csrf`. The session,
token expiry, verified issuer/subject, citizen role, profile readiness and
existing logout/recovery fences are checked. Browser bearer credentials are
rejected. GET follows the existing exact-origin/custom-header read policy.
No browser JWT is returned or stored in client JavaScript.

Neither route accepts a citizen ID, subject, issuer, role, or query parameters.
There are no profile-by-ID endpoints. Ownership always comes from verified
authentication. Staff/service access is not added by this citizen-only API.
All responses use `Cache-Control: no-store`; request logs contain route templates,
request IDs and status/timing only, not profile payloads or credentials.

## GET response

```json
{
  "full_name": "Synthetic Citizen",
  "nic": "123456789v",
  "mobile_number": "+94770000000",
  "email": "synthetic@example.test",
  "name_si": "",
  "name_ta": "",
  "date_of_birth": "",
  "gender": "",
  "address": {"line1": "", "line2": "", "city": ""},
  "revision": 0,
  "has_photo": false,
  "photo_url": null
}
```

`full_name`, `nic` and `mobile_number` come from the existing encrypted
registration record and remain read-only. `email` comes only from a verified
Keycloak access-token claim; it is null when unavailable and may reflect the
token issuance time until refresh. No email/credentials are persisted here.
Issuer, subject, role, encryption keys, credentials and photo object keys are
never response fields.

GA-254 provides authenticated photo upload/retrieval when its optional feature
is enabled. `has_photo` reports a stored reference; `photo_url` is then the
matching authenticated adapter path, otherwise null. See [profile photos](profile-photo.md).
No direct SeaweedFS key or storage credential is exposed.

## PUT request

Send UTF-8 `application/json`. PUT replaces the permitted editable fields;
every key below, including each address key, is mandatory. Empty strings clear
optional values. Omission, null, duplicate keys, unknown fields, case variants,
trailing JSON, and identity/read-only/photo fields are rejected. A maximum
16 KiB body is accepted. Send the revision from the latest GET.

```json
{
  "revision": 0,
  "name_si": "",
  "name_ta": "",
  "date_of_birth": "",
  "gender": "",
  "address": {
    "line1": "Synthetic street",
    "line2": "",
    "city": "Synthetic city"
  }
}
```

Successful PUT returns the same shape as GET, with the saved values and next
revision. Ordinary PUT preserves the photo reference and registration data.
Updates use GA-252's atomic revision predicate. A stale revision returns 409
without changing the database. Reload and reconcile the edit; do not blindly
resubmit an old form or silently discard the user's inputs.

Revisions are nonnegative JSON integers, not strings/fractions/exponents.
Updates accept values through 9007199254740990 so the next revision remains
exactly representable in JavaScript. Names allow 200 Unicode code points each;
address line1/line2/city allow 200/200/100; gender storage allows 64. Invalid UTF-8,
lone surrogate escapes and control characters are rejected. DOB is empty or
`YYYY-MM-DD`, a real calendar date from year 1 through the current UTC date.
These storage bounds do not replace approved product field rules.

## Approved gender and NIC date matching

The approved API gender values are `male` and `female`; future UI labels are
Male and Female. Other nonempty values, including case variants, return 422.
Existing optional values remain clearable using empty strings; approval of the
choices does not make either field mandatory.

Production uses `profile.CitizenFieldRules`. A supplied DOB must match the
registered NIC's year and encoded day. Old 9-digit-plus-V/X numbers use the first
two digits for the year and next three for the day; new 12-digit numbers use
four year digits and three day digits. A day code from 501 to 866 has its 500
offset removed; accepted day codes otherwise range from 001 to 366.

The day comparison uses a fixed 366-day table, reserving February 29 in every
year. Actual DOB validation still rejects February 29 in a non-leap year. Thus
March 1 has code 061 even in a non-leap year, and December 31 has code 366.
Mismatch, impossible date, future date or an unsupported day code returns 422
without saving. `dob_nic_mismatch` identifies correspondence errors;
`invalid_gender` identifies unsupported choices. Gender choices are not additionally matched to the NIC's sex
code; only DOB matching was requested.

An old NIC has no century. Compare the supplied birth year's last two digits;
do not infer 19xx versus 20xx, change identity mappings, or convert NIC numbers.
This validates correspondence, not issuance, checksum, authenticity or identity
ownership. The existing registration/login parser and lookup rules are unchanged.

Encoding references: [SAP Sri Lanka validation rules](https://help.sap.com/docs/successfactors-employee-central/countryregion-specifics/validation-rules-sri-lanka)
and [DRP NIC format FAQ](https://drp.gov.lk/en/faq.php). Boundary tests use
synthetic numbers and cover both formats, both offsets, leap/non-leap dates,
post-February dates, year/day mismatches and invalid codes.

## Status codes

| Status | Meaning |
| --- | --- |
| 200 | Own profile read or permitted update succeeded |
| 400 | Malformed/duplicate/missing/unknown/read-only fields, query, invalid revision |
| 401 | Missing, invalid or expired JWT/browser session; rejected session CSRF binding |
| 403 | Insufficient role/email/profile readiness; origin/CSRF/header/credential policy rejection |
| 404 | Profile/details missing, feature disabled, or unregistered route |
| 409 | Stale profile revision |
| 413 | Update body exceeds 16 KiB |
| 415 | Unsupported media type/encoding |
| 422 | Invalid Unicode/text length/calendar date or an installed field rule rejects input |
| 426 | Direct HTTPS required |
| 429 | Existing browser request limiter applies |
| 503 | Profile/session unavailable, migration/grants failure, or unconfigured field validator |

Errors use `{"code":"revision_conflict","error":"Your profile changed. Reload it before saving again."}`
or another fixed safe message. They never include SQL/provider diagnostics,
other citizens' details, submitted values, storage credentials or tokens.

## Verification and remaining integrations

Tests cover signed synthetic JWT validation, web session/CSRF and expiry/fences,
forged JSON, Unicode/date bounds, owner isolation, stale saves, sanitized errors,
no-cache/logging policy, and persistence through the actual PostgreSQL store in
disposable databases. Live Keycloak, browser UI and physical Android testing are
separate integration work. GA-254 adds photo routes; GA-255/GA-288 consume this
contract; GA-256 verifies complete end-to-end and AI privacy behavior. No profile
data is added to AI requests by these endpoints.
