// Client-side ```spec fence detection + progressive spec assembly.
//
// Assistant messages may interleave prose with fenced ```spec blocks whose body
// is json-render JSONL (nominally one RFC-6902 patch per line). This module
// splits a message's text into an ORDERED list of segments — prose vs spec —
// so the renderer can lay them out inline in the exact order they appeared,
// and feeds each spec block's patches through json-render's patch applier to
// build a live Spec that streams (json-render tolerates missing children
// mid-stream).
//
// Why a hand-rolled state machine instead of core's createMixedStreamParser:
// that parser fires onText/onPatch per line but throws away ORDER and block
// boundaries — it can't tell you "prose, THEN spec A, THEN prose, THEN spec B".
// We need ordered segments (multiple spec blocks per message, prose between
// them), so we own the fence state machine and reuse only the pure
// primitives (parseSpecStreamLine + applySpecPatch) from core.
//
// Streaming discipline: push() buffers a partial trailing line and only
// processes complete `\n`-terminated lines, so a fence marker split across
// chunks (```sp + ec) is reassembled before classification. An unterminated
// fence at stream end is finalized by flush(), which keeps whatever patches
// already parsed (the spec segment stays, half-built).
//
// flush() takes a `final` flag because it has two distinct callers: the live
// re-derive (parseMessageSegments called on the growing message text on every
// SSE delta, `final=false`) and the true end of stream (the turn finishes, or
// a one-shot parse of an already-complete message — history rendering, tests
// — `final=true`, the default). Only `final=true` may report a still-open
// patch accumulator as an error — mid-stream it just means the rest hasn't
// arrived yet, not that anything is actually broken.
//
// One patch != one physical line: the prompt tells the model to emit compact
// single-line JSON per patch (see gen-ui-instructions.mjs), but that's a
// prompt-following assumption, not a guarantee — a real production session
// showed a model pretty-printing its patch JSON across multiple physical
// lines instead. `parseSpecStreamLine`/`JSON.parse` don't care about internal
// whitespace, so the fix is to buffer in-fence text across lines and only
// attempt a parse once the accumulated text is a structurally COMPLETE JSON
// value (brace/bracket depth back to zero) — see `PatchAccumulator` below.
import { applySpecPatch, parseSpecStreamLine } from "@json-render/core";
import type { Spec } from "@json-render/svelte";
import { SPEC_FENCE_CLOSE, SPEC_FENCE_OPEN } from "$lib/common/render";

export const SEGMENT_TEXT = "text";
export const SEGMENT_SPEC = "spec";

// A prose run of the message. `text` accumulates raw characters (newlines
// preserved) as they stream.
export interface TextSegment {
  kind: typeof SEGMENT_TEXT;
  text: string;
}

// A ```spec block. `spec` is the flat json-render spec assembled so far from the
// block's JSONL; `patchCount` is how many patches have applied; `closed` flips
// true when the closing fence is seen (false = still streaming / unterminated).
export interface SpecSegment {
  kind: typeof SEGMENT_SPEC;
  spec: Spec;
  patchCount: number;
  closed: boolean;
}

export type Segment = TextSegment | SpecSegment;

// Callbacks so the caller (conversation store / component) can react to spec
// lifecycle for logging without the fence module importing the logger.
export interface FenceCallbacks {
  onSpecOpen?: () => void;
  onPatch?: (segmentIndex: number, patchCount: number) => void;
  onSpecError?: (message: string) => void;
}

function emptySpec(): Spec {
  return { root: "", elements: {} };
}

// PatchAccumulator buffers in-fence text across an arbitrary number of
// physical lines (and arbitrary chunk boundaries) and reports when the
// accumulated text forms a structurally complete JSON value — i.e. every
// `{`/`[` opened outside a string has been closed and depth returns to zero.
// This is what lets one patch span multiple pretty-printed lines: the common
// case (one compact line = one patch) still completes on the very first
// `feed()` call, so behavior for existing single-line JSONL is unchanged.
//
// The scanner is a small character-by-character state machine:
//   - inString: true while inside a JSON string literal (a `"` that isn't
//     itself escaped toggles this).
//   - escapeNext: true for exactly one character after a `\` inside a
//     string, so an escaped quote (`\"`) doesn't toggle inString and an
//     escaped backslash (`\\`) doesn't leave escapeNext incorrectly armed.
//   - depth: incremented on `{`/`[`, decremented on `}`/`]`, but ONLY when
//     not inString — so `{"content":"a { b } c"}` counts correctly (the
//     braces inside the string are inert).
// depth is allowed to go negative transiently only in genuinely malformed
// input (e.g. a stray leading `}`); isComplete() only fires the moment depth
// returns to exactly 0 after having gone positive, which requires at least
// one real `{`/`[` — pure leading whitespace/garbage never "completes".
class PatchAccumulator {
  private text = "";
  private depth = 0;
  private inString = false;
  private escapeNext = false;
  private sawOpen = false;

  get buffer(): string {
    return this.text;
  }

  get isEmpty(): boolean {
    return this.text.trim() === "";
  }

  // feed appends a chunk of in-fence text (a line, or any substring) and
  // rescans only the newly appended characters, updating depth incrementally.
  feed(chunk: string): void {
    this.text += chunk;

    for (const ch of chunk) {
      if (this.escapeNext) {
        this.escapeNext = false;
        continue;
      }

      if (this.inString) {
        if (ch === "\\") {
          this.escapeNext = true;
        } else if (ch === '"') {
          this.inString = false;
        }
        continue;
      }

      if (ch === '"') {
        this.inString = true;
      } else if (ch === "{" || ch === "[") {
        this.depth += 1;
        this.sawOpen = true;
      } else if (ch === "}" || ch === "]") {
        this.depth -= 1;
      }
    }
  }

  // isComplete reports whether the accumulated text is a structurally closed
  // JSON value: at least one opening brace/bracket has been seen, depth is
  // back to exactly zero, and we're not mid-string.
  isComplete(): boolean {
    return this.sawOpen && this.depth === 0 && !this.inString;
  }

  // isUnrecoverable reports whether the accumulated text can NEVER complete
  // validly: depth has gone negative (a `}`/`]` closed something that was
  // never opened). More accumulation cannot fix this — JSON grammar has no
  // way to "un-close" a bracket — so this is a terminal, not a mid-patch,
  // state. Distinguishes permanently-malformed legacy data (bail out on THIS
  // line immediately) from a merely-still-streaming multi-line patch (keep
  // waiting). Without this check, one malformed line silently swallows every
  // subsequent line in the fence into one never-completing buffer until the
  // closing marker — losing otherwise-valid elements and ballooning the
  // accumulated string across a whole spec block.
  isUnrecoverable(): boolean {
    return this.depth < 0;
  }

  reset(): void {
    this.text = "";
    this.depth = 0;
    this.inString = false;
    this.escapeNext = false;
    this.sawOpen = false;
  }
}

// FenceParser is a stateful, incremental segmenter. Feed it chunks with push()
// and read `segments` after each push to render the current ordered view; call
// flush() once the stream ends to finalize any buffered partial line.
//
// REUSE TRIPWIRE — one-shot only. This class MUTATES `segments` (and the `spec`
// inside each SpecSegment) IN PLACE. That is safe ONLY because the sole live
// caller (`parseMessageSegments`, invoked inside a Svelte `$derived` in
// AssistantContent) builds a FRESH instance per evaluation, reads `segments`
// once, and throws the instance away. Do NOT:
//   - hold a long-lived FenceParser instance across renders, and
//   - read its `segments` reactively (e.g. `$derived(parser.segments)` or a
//     `$state`-wrapped instance).
// Svelte 5 deep-proxies reactive state; an in-place `segment.text += …` or
// `segment.spec = …` on a mutated array element updates a reference the UI
// proxies never re-observe, so the render FREEZES at the first snapshot — the
// exact raw-mutation class of bug fixed in the conversation store (see the
// `assistantId` note there). For incremental / streaming rendering, accumulate
// the raw text elsewhere and RE-RUN `parseMessageSegments` on the full string
// each time (as AssistantContent does) — never mutate a retained instance and
// expect reactivity to see it.
export class FenceParser {
  readonly segments: Segment[] = [];

  private buffer = "";
  private inSpecFence = false;
  private activeSpecIndex = -1;
  private readonly callbacks: FenceCallbacks;
  private readonly patch = new PatchAccumulator();

  constructor(callbacks: FenceCallbacks = {}) {
    this.callbacks = callbacks;
  }

  // push consumes a chunk, processing every complete `\n`-terminated line and
  // holding the trailing partial line in the buffer for the next push.
  push(chunk: string): void {
    this.buffer += chunk;

    const lines = this.buffer.split("\n");
    // The last element is the (possibly empty) partial line after the final \n —
    // keep it buffered until its newline arrives.
    this.buffer = lines.pop() ?? "";

    for (const line of lines) {
      this.processLine(line, true);
    }
  }

  // flush finalizes any buffered partial line. When `final` is true (true
  // stream end), a still-open patch accumulator genuinely will never resolve,
  // so it's reported as an error rather than silently dropped — an entirely
  // untouched accumulator (no patch was ever mid-flight) stays silent, same
  // as today's "unterminated fence keeps whatever already parsed" contract.
  // When `final` is false (a mid-stream re-derive), the SAME "buffer not
  // empty" state just means the rest of the patch hasn't arrived over SSE
  // yet — reporting it would fire once per delta while a large/multi-chunk
  // patch streams in, instead of the zero times it should.
  flush(final = true): void {
    if (this.buffer !== "") {
      this.processLine(this.buffer, false);
      this.buffer = "";
    }

    if (final && !this.patch.isEmpty) {
      this.callbacks.onSpecError?.(
        `unterminated spec line at end of stream: ${this.patch.buffer.trim()}`,
      );
    }
    this.patch.reset();

    if (this.inSpecFence) {
      this.inSpecFence = false;
    }
  }

  // processLine classifies one line. `hadNewline` is false only for the buffered
  // remainder at flush time, so a trailing prose fragment isn't given a spurious
  // newline.
  private processLine(line: string, hadNewline: boolean): void {
    const trimmed = line.trim();

    if (!this.inSpecFence && trimmed.startsWith(SPEC_FENCE_OPEN)) {
      this.openSpec();
      return;
    }

    // Only honor a close-fence marker while no patch is mid-accumulation —
    // a properly-formed patch's depth returns to 0 (accumulator empty) before
    // the closing ``` line arrives in valid output. If depth is NOT zero here,
    // this is a mid-patch fence-close (malformed stream): don't swallow the
    // ``` as JSON content, just discard the incomplete patch and close.
    if (this.inSpecFence && trimmed === SPEC_FENCE_CLOSE) {
      if (!this.patch.isEmpty && !this.patch.isComplete()) {
        this.callbacks.onSpecError?.(
          `spec fence closed mid-patch, discarding incomplete patch: ${this.patch.buffer.trim()}`,
        );
      }
      this.patch.reset();
      this.closeSpec();
      return;
    }

    if (this.inSpecFence) {
      this.applyPatchLine(line, hadNewline);
      return;
    }

    this.appendText(hadNewline ? line + "\n" : line);
  }

  private openSpec(): void {
    this.inSpecFence = true;
    this.patch.reset();
    const segment: SpecSegment = {
      kind: SEGMENT_SPEC,
      spec: emptySpec(),
      patchCount: 0,
      closed: false,
    };
    this.segments.push(segment);
    this.activeSpecIndex = this.segments.length - 1;
    this.callbacks.onSpecOpen?.();
  }

  private closeSpec(): void {
    this.inSpecFence = false;
    const segment = this.segments[this.activeSpecIndex];
    if (segment !== undefined && segment.kind === SEGMENT_SPEC) {
      segment.closed = true;
    }
    this.activeSpecIndex = -1;
  }

  // applyPatchLine feeds one physical in-fence line into the active patch
  // accumulator. A patch may span multiple lines (a model pretty-printing its
  // JSON instead of emitting compact JSONL) — we only attempt to parse once
  // the accumulated text is structurally complete (brace/bracket depth back
  // to zero), which for the common single-line-compact-JSON case happens
  // immediately on the first line, preserving today's behavior exactly.
  private applyPatchLine(line: string, hadNewline: boolean): void {
    const trimmed = line.trim();

    if (this.patch.isEmpty && trimmed === "") {
      // Blank line between patches — nothing accumulated yet, nothing to do.
      return;
    }

    // A line arriving while nothing is accumulated (depth 0) that doesn't
    // even start a JSON value is garbage, not the start of a multi-line
    // patch — depth-tracking alone would never "complete" it (no braces to
    // balance), so it must be reported immediately rather than silently
    // glued onto whatever comes next.
    if (this.patch.isEmpty && !trimmed.startsWith("{")) {
      this.callbacks.onSpecError?.(`unparseable spec line: ${trimmed}`);
      return;
    }

    // Re-append the newline the split() consumed so multi-line JSON keeps its
    // internal whitespace (irrelevant to JSON.parse, but keeps the accumulated
    // buffer faithful for error messages). The buffered flush() remainder has
    // no trailing newline of its own (hadNewline is false there).
    this.patch.feed(hadNewline ? line + "\n" : line);

    if (this.patch.isUnrecoverable()) {
      // Depth went negative — this specific accumulation can never complete,
      // no matter how many more lines arrive. Report THIS line now (matches
      // the old per-line error shape for permanently-malformed data) and
      // reset immediately, rather than dragging every subsequent line in the
      // fence into the same doomed buffer until the closing marker.
      this.callbacks.onSpecError?.(
        `unparseable spec line: ${this.patch.buffer.trim()}`,
      );
      this.patch.reset();
      return;
    }

    if (!this.patch.isComplete()) {
      // Still mid-patch — nothing to parse yet. This is the normal
      // in-progress state for a multi-line patch; not an error.
      return;
    }

    const raw = this.patch.buffer.trim();
    this.patch.reset();

    if (raw === "") {
      return;
    }

    const segment = this.segments[this.activeSpecIndex];
    if (segment === undefined || segment.kind !== SEGMENT_SPEC) {
      return;
    }

    const patch = parseSpecStreamLine(raw);
    if (patch === null) {
      // Structurally-balanced JSON that still failed to parse as a patch
      // (bad op/path shape, or genuinely malformed leftover text) — a real
      // error, not a mid-stream artifact.
      this.callbacks.onSpecError?.(`unparseable spec line: ${raw}`);
      return;
    }

    // applySpecPatch mutates in place; reassign a fresh object so downstream
    // reactive reads (Svelte runes) see a new reference.
    segment.spec = { ...applySpecPatch(segment.spec, patch) };
    segment.patchCount += 1;
    this.callbacks.onPatch?.(this.activeSpecIndex, segment.patchCount);
  }

  private appendText(text: string): void {
    if (text === "") {
      return;
    }

    const last = this.segments[this.segments.length - 1];
    if (last !== undefined && last.kind === SEGMENT_TEXT) {
      last.text += text;
      return;
    }

    this.segments.push({ kind: SEGMENT_TEXT, text });
  }
}

// parseMessageSegments runs a message string through a fresh parser and
// returns the ordered segments. `finalize` (default true) is threaded to
// flush() — pass false when re-deriving segments from a still-growing
// streaming message (see FenceParser.flush's doc comment for why this
// matters), true (or omit) for an already-complete message: history
// rendering, tests, or a live turn once streaming has genuinely finished.
export function parseMessageSegments(
  message: string,
  callbacks: FenceCallbacks = {},
  finalize = true,
): Segment[] {
  const parser = new FenceParser(callbacks);
  parser.push(message);
  parser.flush(finalize);
  return parser.segments;
}
