import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./app/App";
import "@fontsource-variable/inter";
import "./shared/styles/base.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("FocusBoard root element is missing.");
}

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
