# GA-302 presentation contract version 1

Status: implemented shared Go validation and proposed client handoff; design,
backend and GA-289 approval remains pending. No agent enablement or prompt edit.

## Envelope and compatibility

The existing JSON reply retains required nonempty `answer`, existing `sources`
and optional `conversation_id`. An optional `presentation` object contains
`version: 1` and `blocks`. See [JSON Schema](presentation.schema.json),
[synthetic example](testdata/presentation.json) and
[catalog](template-catalog.md). The example is not government guidance.

An absent extension preserves old behavior and old encoded field names exactly.
The Go decoder continues to normalize missing/null sources into an empty array.
Both existing clients explicitly select the answer/source fields and will ignore
this extension until their renderers are implemented. Old Go deployments reject
unknown envelope fields, so upgrade both consuming services before any producer
sends `presentation`. This is additive for updated consumers, not automatically
compatible with every old server. Do not enable agent blocks in this task.

Known envelope keys remain strict: duplicate/unknown root keys, malformed JSON,
invalid UTF-8/surrogates, invalid answer/source/conversation data or an incoming
response above 64 KiB remain errors. Optional data does not relax these guards.

## Blocks

Each block has a unique message-local `id`, a known `type`, optional `title` and
optional `source_numbers`. Omitted references mean no evidence association, not
verified evidence. Source numbers refer to this reply's source numbers, never
array indexes. Numbers must exist, be unique within the reference list and obey
the existing 1–100 integer range. Source metadata is not copied into blocks.

| Type | Required fields | Optional fields beyond common fields |
| --- | --- | --- |
| `text` | `markdown` | None |
| `procedure` | `steps`: items with `id`, `title`, `detail` | Item `source_numbers` |
| `checklist` | `items`: items with `id`, `text` | Item `source_numbers` |
| `table` | `columns`: `id`, `label`; `rows`: arrays of string cells | `caption` |
| `notice` | `severity`: `info` or `warning`; `text` | Paired `date` and `date_label` |
| `faq` | `items`: `id`, `question`, `answer` | Item `source_numbers` |

Citation references are attached using `source_numbers`; no separate invented
document/excerpt/date payload is accepted. Current sources expose title, optional
page and optional URL only. Date/section/excerpt enrichment requires its own
approved source-contract change. Notice dates describe supplied content, not
the date a document was verified. Notice dates must be real YYYY-MM-DD dates
with their supplied label. Invalid dates discard the optional extension.

Table references apply to the whole table in v1, not individual rows. Never
present them as proof of an individual fee. Lists are read-only guidance, not
persisted checkboxes or verified completion states. No recursive blocks.

Limits: at most 20 blocks, 20 items per procedure/checklist/FAQ, 8 columns and
30 rows per table. IDs are 1–64 ASCII letters/digits/underscore/hyphen. IDs must
be unique among blocks and within each block's items/columns. Titles, column
labels, questions and date labels allow 200 Unicode code points; other text
fields allow 4,000; table cells allow 500. Text must contain non-whitespace.
Rows must have exactly as many cells as columns; empty cells are not accepted.
No unknown keys or explicit null values inside v1 structures. Optional reference
arrays may be empty. The whole response still has a 64 KiB UTF-8 byte budget
and answer has its existing 32 KiB byte budget.

JSON Schema covers local shape; Go validation additionally enforces reference
membership, ID uniqueness, rectangular rows, canonical integer lexical form,
Unicode validity and byte budgets. JSON Schema `format: date` needs format
assertion enabled. It is not a substitute for the runtime relational checks.

## Fallback and renderer safety

Unknown version/type, null/empty extension, partial/missing fields, invalid
references or limits discard the entire extension. Decoder returns the unchanged
complete answer and sources with no presentation. Encoder applies the same
validation and removes invalid optional data from its output. If adding a valid
extension exceeds the encoded response budget, encoder emits the legacy reply
without it; an oversized incoming envelope is rejected before parsing.

Valid JSON containing an incomplete block can fall back. Truncated/malformed
JSON cannot: transport must report a failure and preserve existing chat state.
No attempt is made to recover a grounded answer from broken JSON.

Presentation validation is not HTML sanitization or source verification.
`markdown` and other strings can contain HTML-looking text or unsafe URL text;
these remain untrusted data. GA-260/303 must use known React/React Native
components, disable raw HTML, forbid script/embed execution, apply explicit link
policy and render unsafe destinations as text. Never use `dangerouslySetInnerHTML`
or a native WebView for response content. Do not evaluate strings or infer action
payloads from them. Object-level `html`, event handlers, arbitrary styles, URLs
and executable action fields are rejected by this contract.

Render either the accepted presentation or the complete safe answer fallback,
not both as duplicated full responses. Keep the answer available for legacy
clients and readable copy output. An empty answer remains an invalid base reply.
No verified badge, source content or government fact may be fabricated.

## GA-289 extension boundary

Block ordering, message-local IDs and source references provide a future slot
for approved action types. No action payload, map provider, URL policy, credentials
or environment flags are introduced here. `office`, `contact`, `document` and
other unrecognized types currently trigger complete text fallback.

GA-289 must jointly specify its extra types with server-side data validation and
client renderer dispatch; use a later negotiated version if v1 rules must change.
Older clients retain `answer` semantics even when newer clients gain actions.
Unknown actions must never produce clickable buttons. Contact extraction and
action validation are not shifted into this package.

## Verification and rollout

From `backend/pkg/chatcontract`:

```powershell
go test ./...
go vet ./...
```

Tests cover all six templates, multilingual text, source associations, malformed
and unknown structures, invalid dates, duplicate keys/IDs, prohibited executable
fields, Unicode/block/response limits, legacy consumers and round trips.
These are contract tests, not proof of rendered XSS or accessibility protection.

Before producer enablement: obtain team approval; rebuild/test both Go consumers;
implement and test web/mobile safe renderers; provide actual agent source data;
then coordinate producer rollout separately. No live network, credentials,
database migration, prompt changes, map integration or dependency installation
is needed for this contract task.

Verification record (28 November 2026): shared-contract tests and go vet passed
in the repository. Auth-service and core-service suites and go vet passed against
an isolated backend source copy using this contract. Opt-in database and live
external-service integrations were not enabled. JSON Schema validation using
Draft202012Validator with FormatChecker passed for the six-template fixture.
No rendered browser/device security or accessibility result is claimed here.
