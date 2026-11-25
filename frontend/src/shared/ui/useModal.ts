import { useEffect, useRef } from "react";

export function useModal(initialFocus: string) {
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const node = dialog.current;
    const viewport = window.visualViewport;
    const fitViewport = () => {
      if (!node || !viewport) return;
      node.style.setProperty("--task-viewport-height", `${viewport.height}px`);
      node.style.setProperty("--task-viewport-top", `${viewport.offsetTop}px`);
    };
    fitViewport();
    viewport?.addEventListener("resize", fitViewport);
    viewport?.addEventListener("scroll", fitViewport);
    node?.showModal();
    node?.querySelector<HTMLElement>(initialFocus)?.focus();
    return () => {
      viewport?.removeEventListener("resize", fitViewport);
      viewport?.removeEventListener("scroll", fitViewport);
      if (node?.open) node.close();
    };
  }, [initialFocus]);

  return dialog;
}
