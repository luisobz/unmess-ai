// Modal focus follows the active overlay, including nested dialogs.
import { $ } from "./dom.js";

const focusByOverlay = new Map();
const stack = [];
function controls(ov) {
  return [
    ...ov.querySelectorAll(
      "button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex='-1'])",
    ),
  ].filter((el) => el.getClientRects().length > 0);
}
export function openOverlay(name) {
  const ov = $(name + "-overlay");
  if (!ov) return;
  if (ov.hidden) {
    focusByOverlay.set(name, document.activeElement);
    stack.push(name);
  }
  ov.hidden = false;
  controls(ov)[0]?.focus();
}
export function closeOverlay(name) {
  const ov = $(name + "-overlay");
  if (ov) ov.hidden = true;
  const idx = stack.lastIndexOf(name);
  if (idx >= 0) stack.splice(idx, 1);
  const previous = focusByOverlay.get(name);
  focusByOverlay.delete(name);
  if (previous?.isConnected && !previous.closest("[hidden]")) previous.focus();
}
export function initOverlays() {
  document
    .querySelectorAll("[data-close]")
    .forEach((btn) =>
      btn.addEventListener("click", () => closeOverlay(btn.dataset.close)),
    );
  document.querySelectorAll(".overlay").forEach((ov) =>
    ov.addEventListener("click", (e) => {
      if (e.target === ov) closeOverlay(ov.id.replace(/-overlay$/, ""));
    }),
  );
  document.addEventListener("keydown", (e) => {
    const name = stack[stack.length - 1];
    if (!name) return;
    if (e.key === "Escape") {
      e.preventDefault();
      closeOverlay(name);
      return;
    }
    if (e.key !== "Tab") return;
    const list = controls($(name + "-overlay"));
    if (!list.length) {
      e.preventDefault();
      return;
    }
    const first = list[0],
      last = list[list.length - 1];
    if (
      e.shiftKey &&
      (document.activeElement === first ||
        !list.includes(document.activeElement))
    ) {
      e.preventDefault();
      last.focus();
    } else if (
      !e.shiftKey &&
      (document.activeElement === last ||
        !list.includes(document.activeElement))
    ) {
      e.preventDefault();
      first.focus();
    }
  });
}
