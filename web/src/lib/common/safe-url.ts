// URLs in model output are untrusted. Only these schemes become live links;
// anything else (javascript:, data:, vbscript:, file:, ...) renders as text.
const ALLOWED_PROTOCOLS = new Set(["http:", "https:", "mailto:"]);

// safeHref returns the URL when it is an absolute http, https or mailto URL,
// and undefined otherwise. Relative and protocol-relative URLs are refused
// too: model text has no business pointing into this app.
export function safeHref(raw: string | undefined): string | undefined {
  if (raw === undefined) {
    return undefined;
  }

  const trimmed = raw.trim();
  if (trimmed === "" || trimmed.startsWith("//")) {
    return undefined;
  }

  try {
    const url = new URL(trimmed);

    return ALLOWED_PROTOCOLS.has(url.protocol) ? url.href : undefined;
  } catch {
    return undefined;
  }
}
