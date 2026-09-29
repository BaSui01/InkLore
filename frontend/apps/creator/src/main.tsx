import React from "react";
import ReactDOM from "react-dom/client";
import { CreatorApp } from "./App";
import "./styles.css";

const rootElement = document.getElementById("root");
if (rootElement) {
  ReactDOM.createRoot(rootElement).render(
    <React.StrictMode>
      <CreatorApp />
    </React.StrictMode>
  );
}
