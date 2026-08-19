import { describe, it, expect } from "vitest";
import {
  FenceParser,
  parseMessageSegments,
  SEGMENT_SPEC,
  SEGMENT_TEXT,
  type Segment,
  type SpecSegment,
} from "./fence";
import { COMP_CARD, COMP_TEXT } from "$lib/common/render";

// A minimal spec block body (JSONL patches) building a Card with one Text child.
const SPEC_BODY = [
  '{"op":"add","path":"/root","value":"card"}',
  '{"op":"add","path":"/elements/card","value":{"type":"Card","props":{"id":null,"title":"Demo"},"children":["t1"]}}',
  '{"op":"add","path":"/elements/t1","value":{"type":"Text","props":{"id":null,"content":"hello"},"children":[]}}',
].join("\n");

function specSegments(segments: Segment[]): SpecSegment[] {
  return segments.filter((s): s is SpecSegment => s.kind === SEGMENT_SPEC);
}

describe("fence detection — one-shot", () => {
  it("splits prose + spec + prose into ordered segments", () => {
    const message = [
      "Here is your dashboard:",
      "```spec",
      SPEC_BODY,
      "```",
      "Let me know if you need more.",
    ].join("\n");

    const segments = parseMessageSegments(message);

    expect(segments.map((s) => s.kind)).toEqual([
      SEGMENT_TEXT,
      SEGMENT_SPEC,
      SEGMENT_TEXT,
    ]);

    const [before, , after] = segments;
    if (before.kind === SEGMENT_TEXT) {
      expect(before.text).toContain("Here is your dashboard:");
    }
    if (after.kind === SEGMENT_TEXT) {
      expect(after.text).toContain("Let me know if you need more.");
    }
  });

  it("assembles the spec block into the right elements", () => {
    const message = ["```spec", SPEC_BODY, "```"].join("\n");

    const [spec] = specSegments(parseMessageSegments(message));
    expect(spec.closed).toBe(true);
    expect(spec.spec.root).toBe("card");
    expect(Object.keys(spec.spec.elements).sort()).toEqual(["card", "t1"]);
    expect(spec.spec.elements.card.type).toBe(COMP_CARD);
    expect(spec.spec.elements.card.children).toEqual(["t1"]);
    expect(spec.spec.elements.t1.type).toBe(COMP_TEXT);
  });

  it("never leaks raw JSONL or fence markers as a text segment", () => {
    const message = ["prose", "```spec", SPEC_BODY, "```"].join("\n");
    const segments = parseMessageSegments(message);

    for (const seg of segments) {
      if (seg.kind === SEGMENT_TEXT) {
        expect(seg.text).not.toContain('"op"');
        expect(seg.text).not.toContain("```");
      }
    }
  });

  it("handles multiple spec blocks in one message", () => {
    const message = [
      "first",
      "```spec",
      '{"op":"add","path":"/root","value":"a"}',
      "```",
      "second",
      "```spec",
      '{"op":"add","path":"/root","value":"b"}',
      "```",
    ].join("\n");

    const segments = parseMessageSegments(message);
    const specs = specSegments(segments);
    expect(specs).toHaveLength(2);
    expect(specs[0].spec.root).toBe("a");
    expect(specs[1].spec.root).toBe("b");
    expect(segments.map((s) => s.kind)).toEqual([
      SEGMENT_TEXT,
      SEGMENT_SPEC,
      SEGMENT_TEXT,
      SEGMENT_SPEC,
    ]);
  });

  it("keeps an unterminated fence's parsed patches", () => {
    const message = [
      "streaming...",
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      // no closing fence — stream cut off mid-block
    ].join("\n");

    const [spec] = specSegments(parseMessageSegments(message));
    expect(spec.closed).toBe(false);
    expect(spec.spec.root).toBe("card");
  });

  it("returns no segments for an empty message", () => {
    expect(parseMessageSegments("")).toEqual([]);
  });

  it("surfaces an unparseable line inside a fence and keeps going", () => {
    const errors: string[] = [];
    const message = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      "not json at all",
      '{"op":"add","path":"/state","value":{}}',
      "```",
    ].join("\n");

    const [spec] = specSegments(
      parseMessageSegments(message, {
        onSpecError: (m) => errors.push(m),
      }),
    );
    expect(errors).toHaveLength(1);
    expect(spec.spec.root).toBe("card");
    expect(spec.spec.state).toEqual({});
  });
});

describe("fence detection — mid-stream re-derive (finalize=false)", () => {
  // Regression: AssistantContent re-runs parseMessageSegments on the FULL
  // growing message text on every SSE delta while message.streaming is true.
  // Before threading a `finalize` flag through, every one of those
  // intermediate calls called flush() unconditionally, which reported
  // "unterminated spec line at end of stream" any time the trailing patch
  // wasn't structurally complete YET — even though the stream was very much
  // still in flight. A large multi-chunk patch (e.g. a wide table) spammed
  // one spurious error per delta instead of zero.
  const INCOMPLETE_PATCH_MESSAGE = [
    "```spec",
    '{"op":"add","path":"/root","value":"tbl"}',
    // Deliberately cut mid-value — this is what an in-flight streaming
    // message looks like at some arbitrary delta boundary, not a genuinely
    // broken final message.
    '{"op":"add","path":"/elements/tbl","value":{"type":"Table","props":{"id":null,"columns":["A","B"],"rows":[["x1","y1"],["x2","y',
  ].join("\n");

  it("reports nothing when finalize=false, even though the trailing patch is structurally incomplete", () => {
    const errors: string[] = [];
    const segments = parseMessageSegments(
      INCOMPLETE_PATCH_MESSAGE,
      { onSpecError: (m) => errors.push(m) },
      false,
    );

    expect(errors).toEqual([]);
    // The root patch before the cut-off line still applied.
    const [spec] = specSegments(segments);
    expect(spec.spec.root).toBe("tbl");
  });

  it("reports the same buffer as an error once finalize=true (the true end of stream)", () => {
    const errors: string[] = [];
    parseMessageSegments(INCOMPLETE_PATCH_MESSAGE, {
      onSpecError: (m) => errors.push(m),
    });

    expect(errors).toHaveLength(1);
    expect(errors[0]).toContain("unterminated spec line at end of stream");
  });

  it("keeps an incomplete element patch out of the applied spec until it completes", () => {
    const partial = [
      "```spec",
      '{"op":"add","path":"/root","value":"answer"}',
      '{"op":"add","path":"/elements/answer","value":{"type":"Text"',
    ].join("\n");
    const complete = `${partial},"props":{"id":null,"content":"Ready"},"children":[]}}`;

    const [building] = specSegments(parseMessageSegments(partial, {}, false));
    expect(building.spec.elements.answer).toBeUndefined();

    const [rendered] = specSegments(parseMessageSegments(complete, {}, false));
    expect(rendered.spec.elements.answer.type).toBe(COMP_TEXT);
  });

  it("stays silent across every growing prefix of a streamed multi-chunk patch, then parses cleanly once complete", () => {
    // Simulate the real failure: a large JSON value for one patch arrives
    // over many SSE deltas. AssistantContent re-derives on each one with
    // finalize=false; only the true final call (finalize=true, the default)
    // should ever be allowed to report an error — and here the final text is
    // valid, so it shouldn't either.
    const full = [
      "```spec",
      '{"op":"add","path":"/root","value":"tbl"}',
      '{"op":"add","path":"/elements/tbl","value":{"type":"Table","props":{"id":null,"columns":["A","B"],"rows":[["x1","y1"],["x2","y2"]]},"children":[]}}',
      "```",
    ].join("\n");

    const midStreamErrors: string[] = [];
    // Re-derive on growing prefixes, mimicking one call per SSE delta.
    for (let cut = 1; cut < full.length; cut += 7) {
      parseMessageSegments(
        full.slice(0, cut),
        { onSpecError: (m) => midStreamErrors.push(m) },
        false,
      );
    }
    expect(midStreamErrors).toEqual([]);

    const finalErrors: string[] = [];
    const [spec] = specSegments(
      parseMessageSegments(full, { onSpecError: (m) => finalErrors.push(m) }),
    );
    expect(finalErrors).toEqual([]);
    expect(spec.closed).toBe(true);
    expect(spec.spec.elements.tbl).toMatchObject({
      type: "Table",
      props: { columns: ["A", "B"] },
    });
  });

  it("FenceParser.flush(false) suppresses the unterminated-patch error; flush(true) reports it", () => {
    const message = [
      '{"op":"add","path":"/root","value":"x"}',
      '{"op":"add","path":"/elements/y","value":{"type":"Text","props',
    ].join("\n");

    const midErrors: string[] = [];
    const midParser = new FenceParser({
      onSpecError: (m) => midErrors.push(m),
    });
    midParser.push("```spec\n");
    midParser.push(message);
    midParser.flush(false);
    expect(midErrors).toEqual([]);

    const finalErrors: string[] = [];
    const finalParser = new FenceParser({
      onSpecError: (m) => finalErrors.push(m),
    });
    finalParser.push("```spec\n");
    finalParser.push(message);
    finalParser.flush(true);
    expect(finalErrors).toHaveLength(1);
    expect(finalErrors[0]).toContain("unterminated spec line at end of stream");
  });

  // finalize=false must ONLY suppress the ambiguous "trailing patch not
  // complete yet at flush time" signal — it must NOT mask errors that are
  // detectable immediately, regardless of whether more text is still coming.
  // Otherwise a genuinely broken line from the model would go unreported for
  // the entire rest of the stream instead of surfacing on the delta it
  // actually arrived on.
  it("still reports a negative-depth (unrecoverable) line immediately, even mid-stream", () => {
    const message = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      // Over-closed — depth goes negative the instant this line is fed,
      // independent of anything that might arrive afterward.
      '{"op":"add","path":"/elements/bad","value":{"type":"Stat"}}}',
    ].join("\n");

    const errors: string[] = [];
    parseMessageSegments(
      message,
      { onSpecError: (m) => errors.push(m) },
      false,
    );

    expect(errors).toHaveLength(1);
    expect(errors[0]).toContain("unparseable spec line");
  });

  it("still reports a non-JSON garbage line immediately, even mid-stream", () => {
    const message = ["```spec", "not json at all"].join("\n");

    const errors: string[] = [];
    parseMessageSegments(
      message,
      { onSpecError: (m) => errors.push(m) },
      false,
    );

    expect(errors).toHaveLength(1);
    expect(errors[0]).toContain("unparseable spec line");
  });

  it("still reports an explicit fence-close arriving mid-patch, even mid-stream", () => {
    const message = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      '{"op":"add","path":"/elements/broken","value":{',
      "```",
    ].join("\n");

    const errors: string[] = [];
    parseMessageSegments(
      message,
      { onSpecError: (m) => errors.push(m) },
      false,
    );

    expect(errors).toHaveLength(1);
    expect(errors[0]).toContain("mid-patch");
  });
});

describe("fence detection — streaming across chunks", () => {
  it("reassembles a fence marker split across chunks (```sp + ec)", () => {
    const parser = new FenceParser();
    // The opening fence arrives in two chunks with no newline between the halves.
    parser.push("intro\n```sp");
    parser.push('ec\n{"op":"add","path":"/root","value":"card"}\n```\ndone\n');
    parser.flush();

    expect(parser.segments.map((s) => s.kind)).toEqual([
      SEGMENT_TEXT,
      SEGMENT_SPEC,
      SEGMENT_TEXT,
    ]);

    const spec = parser.segments[1];
    if (spec.kind === SEGMENT_SPEC) {
      expect(spec.spec.root).toBe("card");
      expect(spec.closed).toBe(true);
    }
  });

  it("progressively grows a spec as patch lines arrive chunk by chunk", () => {
    const parser = new FenceParser();
    parser.push("```spec\n");
    parser.push('{"op":"add","path":"/root","value":"card"}\n');
    let spec = parser.segments[0];
    if (spec.kind === SEGMENT_SPEC) {
      expect(Object.keys(spec.spec.elements)).toHaveLength(0);
      expect(spec.spec.root).toBe("card");
    }

    parser.push(
      '{"op":"add","path":"/elements/card","value":{"type":"Card","props":{"id":null},"children":[]}}\n',
    );
    spec = parser.segments[0];
    if (spec.kind === SEGMENT_SPEC) {
      expect(Object.keys(spec.spec.elements)).toEqual(["card"]);
    }

    parser.push("```\n");
    parser.flush();
    spec = parser.segments[0];
    if (spec.kind === SEGMENT_SPEC) {
      expect(spec.closed).toBe(true);
    }
  });

  it("fires open + patch callbacks once per open and per patch", () => {
    let opens = 0;
    let patches = 0;
    const parser = new FenceParser({
      onSpecOpen: () => (opens += 1),
      onPatch: () => (patches += 1),
    });
    parser.push("```spec\n");
    parser.push('{"op":"add","path":"/root","value":"x"}\n');
    parser.push('{"op":"add","path":"/state","value":{}}\n');
    parser.push("```\n");
    parser.flush();

    expect(opens).toBe(1);
    expect(patches).toBe(2);
  });
});

describe("fence detection — multi-line pretty-printed patches", () => {
  // Mirrors the real production shape: a faster model pretty-printed its
  // patch JSON across multiple physical lines instead of emitting compact
  // JSONL. parseSpecStreamLine/JSON.parse only ever saw ONE physical line at
  // a time under the old implementation, so a patch like this produced a
  // string of "unparseable spec line" errors (including a bare "}" on its
  // own line) and the element was silently dropped.
  const PRETTY_PRINTED_BLOCK = [
    "```spec",
    '{"op":"add","path":"/root","value":"main"}',
    "{",
    '  "op": "add",',
    '  "path": "/elements/main",',
    '  "value": {',
    '    "type": "Stat",',
    '    "props": {',
    '      "id": null,',
    '      "label": "Placeholder",',
    '      "value": "42",',
    '      "unit": null,',
    '      "delta": null',
    "    },",
    '    "children": []',
    "  }",
    "}",
    "```",
  ].join("\n");

  it("parses a patch pretty-printed across multiple lines with zero onSpecError calls", () => {
    const errors: string[] = [];
    const message = ["intro", PRETTY_PRINTED_BLOCK].join("\n");

    const [spec] = specSegments(
      parseMessageSegments(message, { onSpecError: (m) => errors.push(m) }),
    );

    expect(errors).toEqual([]);
    expect(spec.closed).toBe(true);
    expect(spec.spec.root).toBe("main");
    expect(spec.spec.elements.main).toMatchObject({
      type: "Stat",
      props: { label: "Placeholder", value: "42" },
    });
    expect(spec.patchCount).toBe(2);
  });

  it("ignores structural braces/brackets inside string values when tracking depth", () => {
    const errors: string[] = [];
    const message = [
      "```spec",
      '{"op":"add","path":"/root","value":"c1"}',
      "{",
      '  "op": "add",',
      '  "path": "/elements/c1",',
      '  "value": {',
      '    "type": "Callout",',
      '    "props": {',
      '      "id": null,',
      '      "variant": "info",',
      '      "title": null,',
      // The string value below contains literal { and } and [ and ] — the
      // depth tracker must treat them as inert (inside a string) and not
      // count them toward brace/bracket depth.
      '      "text": "Use the {placeholder} syntax, e.g. [a, b] or {\\"k\\": 1}"',
      "    },",
      '    "children": []',
      "  }",
      "}",
      "```",
    ].join("\n");

    const [spec] = specSegments(
      parseMessageSegments(message, { onSpecError: (m) => errors.push(m) }),
    );

    expect(errors).toEqual([]);
    expect(spec.spec.elements.c1.props.text).toBe(
      'Use the {placeholder} syntax, e.g. [a, b] or {"k": 1}',
    );
  });

  it("still reports onSpecError for a genuinely malformed patch (unbalanced braces that never resolve)", () => {
    const errors: string[] = [];
    const message = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      "{",
      '  "op": "add",',
      '  "path": "/elements/broken",',
      // Deliberately missing the closing braces — this patch never resolves
      // before the fence closes.
      "```",
    ].join("\n");

    const [spec] = specSegments(
      parseMessageSegments(message, { onSpecError: (m) => errors.push(m) }),
    );

    expect(errors).toHaveLength(1);
    expect(errors[0]).toContain("mid-patch");
    // The one well-formed patch before the broken one still applied.
    expect(spec.spec.root).toBe("card");
  });

  // Regression: a real production message had every element end with one
  // extra trailing brace (permanently-malformed legacy data). The old
  // accumulator glued every subsequent line into one never-completing buffer
  // until the fence closed, losing otherwise-valid elements and reporting
  // one giant error instead of one per bad line.
  it("reports one error per over-closed line and still parses valid patches after them", () => {
    const errors: string[] = [];
    const message = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      // Over-closed: one extra `}` at the very end pushes depth negative —
      // this specific line can never complete no matter how much more text
      // follows it.
      '{"op":"add","path":"/elements/bad1","value":{"type":"Stat","props":{"id":null,"label":"A","value":"1"}},"extra":[]}}',
      // A second, independently over-closed line right after it.
      '{"op":"add","path":"/elements/bad2","value":{"type":"Stat","props":{"id":null,"label":"B","value":"2"}},"extra":[]}}',
      // A perfectly valid patch AFTER the two broken ones — must still parse.
      '{"op":"add","path":"/elements/good","value":{"type":"Text","props":{"id":null,"content":"ok"},"children":[]}}',
      "```",
    ].join("\n");

    const [spec] = specSegments(
      parseMessageSegments(message, { onSpecError: (m) => errors.push(m) }),
    );

    // One error PER bad line, not one giant error swallowing everything.
    expect(errors).toHaveLength(2);
    for (const err of errors) {
      expect(err).toContain("unparseable spec line");
    }
    // The root patch (before the bad lines) and the valid patch AFTER the bad
    // lines both applied — the malformed lines didn't poison the rest of the
    // fence.
    expect(spec.spec.root).toBe("card");
    expect(Object.keys(spec.spec.elements)).toContain("good");
    expect(spec.spec.elements.good?.type).toBe(COMP_TEXT);
    expect(spec.closed).toBe(true);
  });

  it("still reports onSpecError for structurally-balanced garbage that fails to parse", () => {
    const errors: string[] = [];
    const message = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      "{ this is not valid json }",
      "```",
    ].join("\n");

    const [spec] = specSegments(
      parseMessageSegments(message, { onSpecError: (m) => errors.push(m) }),
    );

    expect(errors).toHaveLength(1);
    expect(errors[0]).toContain("unparseable spec line");
    expect(spec.spec.root).toBe("card");
  });

  it("still reports onSpecError for a non-JSON garbage line between valid patches (regression: existing behavior)", () => {
    const errors: string[] = [];
    const message = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      "not json at all",
      '{"op":"add","path":"/state","value":{}}',
      "```",
    ].join("\n");

    const [spec] = specSegments(
      parseMessageSegments(message, { onSpecError: (m) => errors.push(m) }),
    );

    expect(errors).toHaveLength(1);
    expect(spec.spec.root).toBe("card");
    expect(spec.spec.state).toEqual({});
  });

  // Regression for the production "multiple graphs" failure: the model dropped
  // the single trailing `}` on two long nested element lines (a BarChart and a
  // DonutChart). The parser used to glue those two lines plus the following
  // valid one into one never-completing buffer, discard all three at the fence
  // close, and leave a root-only spec that rendered as empty_spec. Each such
  // depth-1 truncation must now be salvaged so every element survives.
  it("salvages depth-1 truncated element lines so no element is lost", () => {
    const errors: string[] = [];
    const message = [
      "```spec",
      '{"op":"add","path":"/root","value":"dashboard"}',
      // Missing the final `}` that closes the patch object (value was complete).
      '{"op":"add","path":"/elements/bar","value":{"type":"BarChart","props":{"id":null,"title":"Metrics"},"children":[]}',
      // Same truncation on the next long line.
      '{"op":"add","path":"/elements/donut","value":{"type":"DonutChart","props":{"id":null,"title":"Goroutines"},"children":[]}',
      // A well-formed line after the truncated ones.
      '{"op":"add","path":"/elements/dashboard","value":{"type":"Stack","props":{"id":null,"direction":"vertical","gap":"lg"},"children":["bar","donut"]}}',
      "```",
    ].join("\n");

    const [spec] = specSegments(
      parseMessageSegments(message, { onSpecError: (m) => errors.push(m) }),
    );

    // Every element recovered; the root resolves to a defined element.
    expect(errors).toEqual([]);
    expect(spec.spec.root).toBe("dashboard");
    expect(Object.keys(spec.spec.elements).sort()).toEqual([
      "bar",
      "dashboard",
      "donut",
    ]);
    expect(spec.spec.elements.bar?.type).toBe("BarChart");
    expect(spec.spec.elements.donut?.type).toBe("DonutChart");
    expect(spec.closed).toBe(true);
  });

  it("assembles an identical result regardless of how the multi-line patch is chunked across push() calls", () => {
    // Simulate SSE delta boundaries splitting the same logical text at
    // arbitrary points — mid-line, mid-token, mid-brace — and confirm the
    // final result is identical to feeding it as one shot.
    const wholeMessage = ["intro", PRETTY_PRINTED_BLOCK].join("\n");

    const oneShotErrors: string[] = [];
    const oneShot = parseMessageSegments(wholeMessage, {
      onSpecError: (m) => oneShotErrors.push(m),
    });

    function chunkAt(text: string, sizes: number[]): string[] {
      const chunks: string[] = [];
      let i = 0;
      for (const size of sizes) {
        chunks.push(text.slice(i, i + size));
        i += size;
      }
      chunks.push(text.slice(i));
      return chunks.filter((c) => c !== "");
    }

    const chunkPlans: number[][] = [
      [1], // one char, then the rest
      [3, 7, 2], // small arbitrary chunks
      [40, 15, 60], // larger arbitrary chunks that land mid-token/mid-brace
    ];

    for (const plan of chunkPlans) {
      const errors: string[] = [];
      const parser = new FenceParser({ onSpecError: (m) => errors.push(m) });
      for (const chunk of chunkAt(wholeMessage, plan)) {
        parser.push(chunk);
      }
      parser.flush();

      expect(errors).toEqual(oneShotErrors);

      const chunkedSpec = specSegments(parser.segments)[0];
      const oneShotSpec = specSegments(oneShot)[0];
      expect(chunkedSpec.spec).toEqual(oneShotSpec.spec);
      expect(chunkedSpec.patchCount).toBe(oneShotSpec.patchCount);
      expect(chunkedSpec.closed).toBe(oneShotSpec.closed);
    }
  });
});
