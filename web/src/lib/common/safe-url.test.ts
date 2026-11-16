import { describe, expect, it } from "vitest";
import { safeHref } from "./safe-url";

describe("safeHref", () => {
  it("keeps http, https and mailto URLs", () => {
    expect(safeHref("https://example.com/a?b=1")).toBe(
      "https://example.com/a?b=1",
    );
    expect(safeHref("http://example.com")).toBe("http://example.com/");
    expect(safeHref("mailto:someone@example.com")).toBe(
      "mailto:someone@example.com",
    );
  });

  it("refuses script, data and other schemes", () => {
    expect(safeHref("javascript:alert(1)")).toBeUndefined();
    expect(safeHref(" JavaScript:alert(1)")).toBeUndefined();
    expect(safeHref("data:text/html,<b>x</b>")).toBeUndefined();
    expect(safeHref("vbscript:msgbox(1)")).toBeUndefined();
    expect(safeHref("file:///etc/passwd")).toBeUndefined();
  });

  it("refuses relative, protocol-relative and empty URLs", () => {
    expect(safeHref("/api/v1/chats")).toBeUndefined();
    expect(safeHref("//evil.example")).toBeUndefined();
    expect(safeHref("")).toBeUndefined();
    expect(safeHref(undefined)).toBeUndefined();
  });
});
