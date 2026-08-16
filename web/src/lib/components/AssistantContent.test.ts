import { describe, it, expect, vi, afterEach } from "vitest";
import { render } from "@testing-library/svelte";
import { log } from "$lib/log";
import AssistantContent from "./AssistantContent.svelte";

// Regression: AssistantContent re-derives its segments from the FULL
// accumulated text on every SSE delta (FenceParser is stateless-per-call by
// design). Left unguarded, every structural callback (onSpecOpen/onPatch/
// onSpecError) — and the render-summary effect — refires for content ALREADY
// reported on a prior delta, not just the newly-arrived increment: one open
// spec block logs spec.open again on every single delta for the rest of the
// stream. These tests drive the component through a growing-text sequence
// (simulating deltas via rerender) and assert each distinct event logs
// exactly once, while a genuinely NEW occurrence on a later delta still logs.
describe("AssistantContent — dedupes replay-triggered logging across re-derives", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("logs spec.open once across many re-derives of the same still-open block", async () => {
    const infoSpy = vi.spyOn(log, "info").mockImplementation(() => {});

    const growing = [
      "```spec\n",
      '```spec\n{"op":"add","path":"/root","value":"card"}\n',
      // still mid-patch on a second, multi-line patch — nothing new completes
      '```spec\n{"op":"add","path":"/root","value":"card"}\n{\n  "op": "add",\n',
    ];

    const { rerender } = render(AssistantContent, {
      props: { text: "", streaming: true },
    });

    for (const text of growing) {
      // eslint-disable-next-line no-await-in-loop
      await rerender({ text, streaming: true });
    }

    const specOpenCalls = infoSpy.mock.calls.filter(
      (c) => c[0] === "spec.open",
    );
    expect(specOpenCalls).toHaveLength(1);
  });

  it("logs spec.patch once per distinct count, not once per re-derive", async () => {
    const debugSpy = vi.spyOn(log, "debug").mockImplementation(() => {});

    const step1 = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
    ].join("\n");
    const step2 = [
      step1,
      '{"op":"add","path":"/elements/card","value":{"type":"Text","props":{"id":null,"content":"hi"},"children":[]}}',
    ].join("\n");

    const { rerender } = render(AssistantContent, {
      props: { text: step1, streaming: true },
    });

    // Re-derive the SAME text (as if a delta arrived that didn't complete a
    // new patch) — must not refire for count 1 again.
    await rerender({ text: step1, streaming: true });
    await rerender({ text: step1, streaming: true });

    let patchCalls = debugSpy.mock.calls.filter((c) => c[0] === "spec.patch");
    expect(patchCalls).toHaveLength(1);
    expect(patchCalls[0][1]).toMatchObject({ count: 1 });

    // A genuinely new patch completing on a later delta DOES still log.
    await rerender({ text: step2, streaming: true });

    patchCalls = debugSpy.mock.calls.filter((c) => c[0] === "spec.patch");
    expect(patchCalls).toHaveLength(2);
    expect(patchCalls[1][1]).toMatchObject({ count: 2 });
  });

  it("logs spec.render once per distinct (elements, closed) shape", async () => {
    const debugSpy = vi.spyOn(log, "debug").mockImplementation(() => {});

    const open = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      '{"op":"add","path":"/elements/card","value":{"type":"Text","props":{"id":null,"content":"hi"},"children":[]}}',
    ].join("\n");
    const closed = [open, "```"].join("\n");

    const { rerender } = render(AssistantContent, {
      props: { text: open, streaming: true },
    });
    await rerender({ text: open, streaming: true });
    await rerender({ text: open, streaming: true });

    let renderCalls = debugSpy.mock.calls.filter((c) => c[0] === "spec.render");
    expect(renderCalls).toHaveLength(1);
    expect(renderCalls[0][1]).toMatchObject({ elements: 1, closed: false });

    // Closing the fence changes the shape (closed flips true) — logs once more.
    await rerender({ text: closed, streaming: false });

    renderCalls = debugSpy.mock.calls.filter((c) => c[0] === "spec.render");
    expect(renderCalls).toHaveLength(2);
    expect(renderCalls[1][1]).toMatchObject({ elements: 1, closed: true });
  });

  it("logs spec.error once for a garbage line even though it survives many later re-derives", async () => {
    const errorSpy = vi.spyOn(log, "error").mockImplementation(() => {});

    const withGarbage = [
      "```spec",
      '{"op":"add","path":"/root","value":"card"}',
      "not json at all",
    ].join("\n");
    // The fence closes cleanly right after the garbage line — no NEW
    // problem is introduced by this growth, so no new error should log.
    const withCloseAfter = [withGarbage, "```"].join("\n");

    const { rerender } = render(AssistantContent, {
      props: { text: withGarbage, streaming: true },
    });
    await rerender({ text: withGarbage, streaming: true });

    let errorCalls = errorSpy.mock.calls.filter((c) => c[0] === "spec.error");
    expect(errorCalls).toHaveLength(1);

    // The fence closing after the (already-reported) garbage line must not
    // re-log it.
    await rerender({ text: withCloseAfter, streaming: false });

    errorCalls = errorSpy.mock.calls.filter((c) => c[0] === "spec.error");
    expect(errorCalls).toHaveLength(1);
  });
});

describe("AssistantContent — GenUI stream reveal", () => {
  it("keeps an incomplete child invisible until the real component is complete", async () => {
    const partial = [
      "```spec",
      '{"op":"add","path":"/root","value":"stack"}',
      '{"op":"add","path":"/elements/stack","value":{"type":"Stack","props":{"id":null,"direction":"vertical","gap":"md"},"children":["answer"]}}',
      '{"op":"add","path":"/elements/answer","value":{"type":"Text"',
    ].join("\n");
    const complete = `${partial},"props":{"id":null,"content":"Ready"},"children":[]}}`;

    const { rerender } = render(AssistantContent, {
      props: { text: partial, streaming: true },
    });

    const stack = document.querySelector('[data-jr-type="Stack"]');
    expect(stack).not.toBeNull();
    expect(stack?.querySelector('[data-jr-type="Text"]')).toBeNull();
    expect(document.querySelector("[data-testid=spec-build]")).toBeNull();

    await rerender({ text: complete, streaming: true });

    expect(stack?.querySelector('[data-jr-type="Text"]')).toHaveTextContent(
      "Ready",
    );
  });
});
