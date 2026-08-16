// Svelte action for popover/dropdown panels anchored via CSS (left:0 or
// right:0 relative to a trigger). A trigger near a screen edge can still push
// its anchored panel off-screen — e.g. a right-anchored settings panel whose
// trigger sits near the LEFT edge on a narrow phone. This measures the
// panel's actual rendered position after layout and nudges it back on-screen
// with a translateX, the same idea as a desktop context menu flipping side
// when there's no room to open in its default direction.
const EDGE_MARGIN_PX = 8;

export function clampToViewport(node: HTMLElement) {
  function reposition(): void {
    // Reset before measuring so a stale nudge from a prior open/resize
    // doesn't compound into the new measurement.
    node.style.transform = "";

    const rect = node.getBoundingClientRect();
    let shiftX = 0;

    if (rect.left < EDGE_MARGIN_PX) {
      shiftX = EDGE_MARGIN_PX - rect.left;
    } else if (rect.right > window.innerWidth - EDGE_MARGIN_PX) {
      shiftX = window.innerWidth - EDGE_MARGIN_PX - rect.right;
    }

    if (shiftX !== 0) {
      node.style.transform = `translateX(${shiftX}px)`;
    }
  }

  // The panel just mounted this tick; wait for layout so getBoundingClientRect
  // reflects its final CSS-anchored position before measuring.
  requestAnimationFrame(reposition);

  window.addEventListener("resize", reposition);

  return {
    destroy(): void {
      window.removeEventListener("resize", reposition);
    },
  };
}
