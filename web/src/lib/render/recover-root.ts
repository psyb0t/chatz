// Frontend root recovery.
//
// json-render resolves the tree from spec.root and draws NOTHING when that key
// names no element in the map — the failure is a silent blank block (see
// Renderer.svelte). The model produces this by parroting the prompt's example
// root literal ("main") while keying its actual top-level container something
// else ("stackMain"), so /root dangles even though a perfectly good tree was
// emitted. @json-render/core's autoFixSpec deliberately does NOT rewire root
// (too risky in the general case), so chatz recovers it here.
//
// A flat spec's real root is the one element no other element lists among its
// children. When that is unambiguous (exactly one unreferenced element), we
// rewire spec.root to it. When it is ambiguous (zero unreferenced elements — a
// cycle — or more than one — disconnected trees), we leave the spec untouched
// and let validateSpec surface the error rather than guess wrong.
import type { Spec, UIElement } from "@json-render/svelte";

// recoverRoot returns a Spec whose root resolves to a defined element. If the
// current root already resolves it returns the spec unchanged; otherwise it
// rewires root to the sole unreferenced element when there is exactly one, and
// returns the spec unchanged when recovery would be a guess.
export function recoverRoot(spec: Spec): Spec {
  const elements = spec.elements ?? {};
  const keys = Object.keys(elements);
  if (keys.length === 0) {
    return spec;
  }

  if (spec.root !== undefined && elements[spec.root] !== undefined) {
    return spec;
  }

  const referenced = new Set<string>();
  for (const element of Object.values(elements) as UIElement[]) {
    for (const child of element.children ?? []) {
      referenced.add(child);
    }
  }

  const unreferenced = keys.filter((key) => !referenced.has(key));
  if (unreferenced.length !== 1) {
    return spec;
  }

  return { ...spec, root: unreferenced[0] };
}
