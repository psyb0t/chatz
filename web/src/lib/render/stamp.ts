// Frontend id stamping.
//
// The demo/LLM emits every element's `props.id` as null (the catalog declares
// id: z.string().nullable(), and json-render does NOT auto-stamp ids). Because
// chatz has no server-side spec-finalize step (fence detection is fully
// client-side — see chatz-json-render-custom-registry memory), the FRONTEND
// allocates ids: we walk `elements` and, wherever props.id is null/absent, set
// it to that element's map KEY. The key is stable + unique within a spec, so
// this yields a deterministic, addressable DOM id on every node without needing
// UUIDs. A future option is to move stamping server-side (brain does this via a
// UI-gen finalize that allocates UUIDs); for chatz's client-only pipeline,
// stamping here is the chosen approach.
import type { Spec, UIElement } from "@json-render/svelte";

const PROP_ID = "id";

// stampSpecIds returns a new Spec whose every element has a non-null props.id,
// defaulting to the element's map key. Elements are cloned shallowly so the
// input spec is not mutated; unrelated props are preserved untouched.
export function stampSpecIds(spec: Spec): Spec {
  const elements: Record<string, UIElement> = {};

  for (const [key, element] of Object.entries(spec.elements)) {
    const props: Record<string, unknown> = { ...element.props };
    if (props[PROP_ID] === null || props[PROP_ID] === undefined) {
      props[PROP_ID] = key;
    }

    elements[key] = { ...element, props };
  }

  return { ...spec, elements };
}
