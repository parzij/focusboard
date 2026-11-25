import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { App } from "./App";

describe("App", () => {
  it("renders the FocusBoard heading inside the main landmark", () => {
    expect(renderToStaticMarkup(<App />)).toBe(
      "<main><h1>FocusBoard</h1></main>",
    );
  });
});
