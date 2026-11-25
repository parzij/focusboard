import type { JSX as ReactJSX } from "react";

declare global {
  // dnd-kit 6/10 references JSX.Element from before React 19 scoped it.
  namespace JSX {
    type Element = ReactJSX.Element;
    type IntrinsicElements = ReactJSX.IntrinsicElements;
  }
}
