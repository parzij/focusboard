import { useEffect, type RefObject } from "react";

export function useDragScroll(
  board: RefObject<HTMLDivElement | null>,
  enabled: boolean,
) {
  useEffect(() => {
    const element = board.current;
    if (!element || !enabled) return;
    let point: { x: number; y: number } | undefined;
    let frame = 0;

    function update() {
      const rect = element!.getBoundingClientRect();
      if (point && point.y >= rect.top && point.y <= rect.bottom) {
        const left = Math.max(0, rect.left);
        const right = Math.min(window.innerWidth, rect.right);
        const edge = Math.min(64, (right - left) * 0.18);
        if (point.x >= left && point.x <= right && edge > 0) {
          const distance =
            point.x - left < edge
              ? -(left + edge - point.x)
              : Math.max(0, point.x - (right - edge));
          if (distance) element!.scrollLeft += (distance / edge) * 18;
        }
      }
      frame = requestAnimationFrame(update);
    }

    function mouse(event: MouseEvent) {
      point = { x: event.clientX, y: event.clientY };
    }
    function touch(event: TouchEvent) {
      const finger = event.touches[0];
      point = finger && { x: finger.clientX, y: finger.clientY };
    }

    window.addEventListener("mousemove", mouse, { passive: true });
    window.addEventListener("touchmove", touch, { passive: true });
    frame = requestAnimationFrame(update);
    return () => {
      window.removeEventListener("mousemove", mouse);
      window.removeEventListener("touchmove", touch);
      cancelAnimationFrame(frame);
    };
  }, [board, enabled]);
}
