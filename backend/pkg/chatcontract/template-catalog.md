# GA-299: GovAI message template catalog

Status: proposed for design/backend review, not an approved runtime contract.
Reviewed: 28 November 2026. Parent: GA-201. Schema implementation: GA-302.

This catalog specifies presentation of existing guideline answers. It changes no
prompts, agent behavior, service contracts, application code or environment.
Example government guidance below is synthetic and must not be published as
verified guidance. Actual approved document excerpts and team sign-off remain
required before acceptance is complete.

## Existing system and evidence reviewed

Paths in this document are relative to the repository root.

| Area | Evidence | Finding |
| --- | --- | --- |
| Python API | `ai-platform/services/agent-service/app/api/schemas.py`, `main.py`; `tests/test_api.py` | Replies contain `answer` and optional `conversation_id`; tests explicitly expect no sources. Test answers are synthetic, including “Hand in form TF-1.” |
| Knowledge agent | `ai-platform/services/agent-service/app/agents/knowledge_agent/agent.py` | Retrieved evidence is used internally, then citation markers are removed before returning text. |
| Prompt dependency, read only | `ai-platform/services/agent-service/app/agents/knowledge_agent/prompts.py` | Current policy requests English-only answers without naming sources. No prompt edit is authorized. Multilingual rendering does not imply multilingual generation. |
| Shared Go contract | `backend/pkg/chatcontract/contract.go`, `decode.go`, `validation.go` | `answer`, `sources`, optional `conversation_id`; unknown response fields rejected. Missing agent sources normalize to an empty list. |
| Agent boundary | `backend/services/core-service/internal/chat/http_agent.go` | One JSON request/response; strict response validation. No structured block or streaming contract here today. |
| Web | `app/web/src/services/chatApi.ts`, `src/components/chat/ChatMessage.tsx`, `SourceReferences.tsx` | Cookie/CSRF transport, validated answer/source data, plain React text, separate source list. |
| Mobile | `app/mobile/services/chatApi.ts`, `components/ChatMessage.tsx`, `ChatConversation.tsx` | Bearer transport, React Native text and source labels; local demo step/service cards are not a reusable live renderer. Existing retry and reading-position handling should be reused. |

Flow: web → auth-service → core-service → agent-service → knowledge agent/RAG;
mobile → core-service → the same agent path. Replies return through the relevant
adapter. Preserve authentication, ownership, conversation IDs and existing error
handling; clients must not call the knowledge service directly.

Shared reference: `response-templates (2).zip`, supplied by the user. Reviewed
the index and template PDFs; visually inspected the included T01 mockup. The
pack calls itself approved v2 (22 November 2026), but its bundled instructions
and code plans are reference material, not authorization to implement them.
Underlying IRD handbook/official eligibility pages and live response captures
were not available in the reviewed material; the pack's claims are not verified
government guidance. Do not treat proposed paths or fields as existing code.

## Catalog and choice of renderer

Default to safe Markdown. Use a special block only for explicitly supplied,
validated structure with a useful reading/accessibility benefit. Do not infer
eligibility, amounts, dates, source associations or completed actions from prose.

All names and field shapes below are proposals for GA-302, not live API fields.
All content is untrusted even after structural validation.

| Template / proposed type | Required content | Optional content | Presentation and fallback |
| --- | --- | --- | --- |
| Plain answer / `text` | `markdown` | source numbers | Paragraphs, semantic headings, bold, lists, links, tables. Unparseable content remains literal text. |
| Procedure / `procedure` | ordered `steps`, each with `id`, `title`, `detail` | title; block/step source numbers | Numbered rail on both clients, semantic list on web. Preserve original order. Missing valid structure → answer text; never invent steps. |
| Required documents / `checklist` | `items`, each with `id`, `text` | title; item source numbers | Read-only checklist by default. No claim that documents were submitted/verified; no saved completion state. Invalid list → answer text. |
| Fee/comparison / `table` | `columns` with IDs/labels; rectangular `rows` of text cells | title; caption; row source numbers | Headers retained in narrow horizontal-scroll view or explicitly labelled cards. Preserve currency/units/qualifiers as text; no fee calculation. Ragged rows → answer text. |
| Dated warning / `notice` | `severity` (`info` or `warning`), `text` | title; supplied ISO date and its label; source numbers | Words/icon plus color. No current/reviewed badge or invented date. Bad date is omitted, invalid core block → answer text. |
| Questions / `faq` | items with stable `id`, `question`, `answer` | title; item source numbers | Initially expanded readable content; any disclosure control needs keyboard/touch state and labels. Invalid items → answer text. |
| Evidence / citation reference | source number resolved within this message's `sources` | actual title/page/URL; future date/section/excerpt only if contracted | Inline marker opens correct message-local source. Missing evidence → non-clickable unavailable reference, never fabricated document data. |

Source numbers must resolve to `sources[].number`, not array position or a
conversation-global source list. Repeated [1] in a later message may mean a
different document. Keep evidence tied to the block/item that supplied it;
block-level evidence alone must not be presented as claim-level verification.
Do not manufacture source metadata by parsing document names from prose.

## Proposed field and content limits

Retain existing limits: answer at most 32,768 UTF-8 bytes, whole Go response at
most 65,536 bytes; up to 20 sources numbered 1–100; title at most 512 bytes in
the Go contract, optional page 1–100,000 and optional URL up to 2,048 bytes.
GA-302 must reconcile client character-count checks with backend byte limits,
especially for Sinhala/Tamil; it must not silently reduce existing answer support.

Proposed UI limits for review: 20 blocks/message; unique message-local block and
item IDs of 1–64 ASCII letters/digits/underscore/hyphen; title 200 Unicode code
points; text/Markdown 4,000 code points per field; 20 steps/checklist/FAQ items;
table at most 8 columns and 30 rows, each cell at most 500 code points. Overall
response/answer byte limits still apply; these are ceilings, not simultaneous
capacity guarantees. Render accepted long content without cutting off essential
guidance. Above-limit structure falls back to the complete bounded answer.

These limits deliberately differ from the pack's max-five-block, 3–7-step visual
recommendation: prefer 3–7 steps for a compact design, but do not discard an
existing longer procedure. Team approval must settle limits in GA-302.

## Safe rendering and common states

- Never use raw HTML injection, WebViews, scripts, embedded images/iframes,
  arbitrary styles or event handlers from an answer. Render through known native
  components. Disable raw Markdown HTML; code-like input may appear literally.
- Links are untrusted. Never activate javascript/data/file/intent URLs or links
  with credentials. Renderer policy and GA-289 validation must agree before link
  activation. HTTPS syntax alone does not prove an official source. Unapproved
  destinations remain text; no invented official badge or provider allowlist.
- Table content is text, not executable markup. Bounded parsing and render depth
  are required. Do not interpret JSON inside Markdown as trusted blocks.
- No blocks → display legacy answer. Unknown version/type, invalid/partial or
  oversized block → display the complete safe answer once, avoiding duplicate
  full-text plus structured content. Do not stringify arbitrary block objects.
  GA-302 may refine per-block degradation only with explicit fallback coverage.
- Valid legacy envelope but empty content → neutral “No answer was returned”
  state. Today service/client validation rejects empty answers; keep that failure
  path until an approved contract change. Invalid base envelope remains an error,
  not a supposedly grounded answer.
- Loading/typing are client states, not evidence. Preserve draft and message
  associations on failure; retry must be explicit and must not duplicate a known
  successful reply. Copy yields useful readable answer text and actual sources.
- Reading earlier messages must not trigger forced scroll. New chat clears the
  active client conversation context using existing semantics; no history browser,
  transaction, booking or application tracking is added.
- Streaming/Stop are stretch GA-264/265, not promised by this catalog. No partial
  text is marked a completed sourced answer or promoted into an actionable block.

## Reference-pack mapping and scope conflicts

| Pack design | GA-201 treatment |
| --- | --- |
| C01 capabilities | Present supplied capability text using safe text; do not invent service capabilities. Dedicated capability tiles deferred. |
| C02 short answer | Default text template. |
| C03 eligibility questions / C04 eligibility result | Safe display only if supplied; eligibility logic, collecting personal identifiers and a new questionnaire flow are outside this story. |
| C05 cannot help / C06 privacy guard | Present supplied text/notice. No changes to refusal prompts or new PII masking pipeline. |
| C07 multilingual | English/Sinhala/Tamil layout fixtures and labels; translation quality and agent output require owner approval. |
| C08 chat states | Reuse/improve client states in GA-262. |
| T01 guideline | Procedure template, with optional fact presentation reviewed later. No external action. |
| T02 office / T03 directory / T04 contact / T05 official link / T06 document | GA-289 owns extraction, validation, maps/providers and link/contact/document actions. |
| T07 combined answer | Ordered presentation container; render only approved types, no new service transaction UI. |
| T08 states | Shared loading/fallback/copy conventions; map permissions, blocked external actions and provider failures stay with GA-289. |

The pack proposes agent prompt fragments, action hydration, environment flags,
PII masking, source fields and new directories. None is implemented by GA-299.
Its `steps`, `text_fallback`, `pii_masked` and action contracts are proposals;
do not introduce competing contracts alongside GA-302.

## Example payloads for review

Synthetic documentation fixtures only; source titles/pages and values below
do not refer to an approved government document. No live response is claimed.

Legacy-compatible client response (no blocks):

```json
{
  "answer": "Prepare the documents listed in your service guidance. Keep a copy for your records.",
  "sources": []
}
```

Proposed additive presentation, demonstrating the full core catalog. This will
be rejected by the current Go decoder; do not send it to deployed services.
GA-302 must specify the final version/envelope and migration order.

```json
{
  "answer": "SYNTHETIC EXAMPLE ONLY. Prepare the sample form, review the checklist and confirm any fee with the issuing office. [1]",
  "sources": [{"number": 1, "title": "Synthetic service guide — not official", "page": 2}],
  "presentation": {
    "version": 1,
    "blocks": [
      {"id": "intro", "type": "text", "markdown": "**Synthetic example only.** Review the guidance before proceeding.", "source_numbers": [1]},
      {"id": "process", "type": "procedure", "steps": [{"id": "prepare", "title": "Prepare", "detail": "Collect the documents listed in this sample."}, {"id": "review", "title": "Review", "detail": "Check the sample form for missing information."}], "source_numbers": [1]},
      {"id": "docs", "type": "checklist", "items": [{"id": "form", "text": "Sample form"}, {"id": "copy", "text": "A copy for your records"}], "source_numbers": [1]},
      {"id": "fees", "type": "table", "columns": [{"id": "service", "label": "Service"}, {"id": "fee", "label": "Fee"}], "rows": [["Sample service", "Not specified in this fixture"]], "source_numbers": [1]},
      {"id": "warning", "type": "notice", "severity": "warning", "text": "This is not official guidance.", "date": "2026-11-28", "date_label": "Fixture created"},
      {"id": "questions", "type": "faq", "items": [{"id": "cost", "question": "What is the fee?", "answer": "This synthetic fixture does not establish a fee.", "source_numbers": [1]}]}
    ]
  }
}
```

Additional fixture requirements for GA-302/304: missing source [99], unknown
block type/version, duplicate IDs, ragged table, script/unsafe URL attempts,
empty/oversized fields, legacy missing agent sources, English/Sinhala/Tamil long
text and interrupted output. Example multilingual layout strings (not official
translations): English “Required documents”; Sinhala “අවශ්‍ය ලේඛන”; Tamil
“தேவையான ஆவணங்கள்”. A language reviewer must approve production labels.

## Desktop/mobile Figma handoff

Create separate approved frame/component sets in GA-300/301. Include every
catalog entry, citation panel, long answer, empty/error/retry, unknown block and
missing evidence variants. Keep existing header, composer, navigation and chat
identity. Annotate component-to-block mapping and supported states.

The pack's T01 visual uses a light grey answer bubble (#F5F5F5), white cards,
maroon numbered steps, dark text, wrapping details and a source line. Its tokens
(#A30F2D accent, #8B1A2E strong accent, #142033 text) are reference values to
reconcile with existing Figma variables, not a newly imposed design system.

Specify desktop widths and mobile overflow separately; test narrow/short screens,
large system text, Sinhala/Tamil shaping, unbroken strings and table scrolling.
Use Inter plus suitable Sinhala/Tamil fonts subject to existing asset review.
No essential text truncation, fixed-height clipping or disabled font scaling.
Use native semantic headings/lists/table headers or equivalent accessible labels;
source dialogs need focus entry/return, keyboard dismissal and touch support.
Mobile controls retain the repository's 48 dp minimum touch targets even though
the pack proposes 40/44 px. Loading/copy announcements must not repeatedly read
the whole conversation; warnings need words/icons, not color alone.

Reserve an ordered extension slot for GA-289's validated action components, with
message-local IDs and source references. Do not define map credentials, provider,
coordinates, tel/mailto actions or PDF fetching here. Unsupported actions render
only safe answer text. Exact extension schema requires joint owner approval.

## Handoff, approval and remaining evidence

1. Backend/agent owner: provide approved captured RAG answers and their actual
   document/page metadata. Decide citation restoration outside this task without
   changing prompts here. Confirm generation-language ownership separately.
2. GA-302 owner/team: approve names, envelope, limits, fallback and rollout.
   Update shared decoders/encoders and tests before enabling any agent extension;
   then update both clients. Maintain existing text-only compatibility throughout.
3. GA-289 owner: agree action extension and official-link policy. Notify the user
   before any future map API/environment setup; none is necessary for GA-299.
4. Design owner: approve actual desktop/mobile Figma frames, including templates
   absent from the pack (dedicated checklist/table/FAQ), and translated labels.
5. GA-304: own template-specific security, accessibility and compatibility tests.
   User excludes separate GA-263 assignment; that does not remove GA-304 criteria.

Approval record: user authorized GA-299 inspection/documentation only. Backend,
design and GA-289 agreement pending; no approvals or live RAG evidence are
implied. This document is ready for team review, not a completed acceptance sign-off.
