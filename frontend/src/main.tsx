import React from "react";
import { createRoot } from "react-dom/client";
import "@unocss/reset/tailwind.css";
import "virtual:uno.css";
import "./style.css";
import "./i18n";

const container = document.getElementById("root");

const root = createRoot(container!);

const isStartupWindow = window.location.pathname.startsWith("/startup");

async function mountApplication() {
  const { default: ApplicationRoot } = isStartupWindow
    ? await import("./components/startup/StartupWindow")
    : await import("./App");

  root.render(
    <React.StrictMode>
      <ApplicationRoot />
    </React.StrictMode>,
  );

  container?.classList.add("ready");
}

void mountApplication();
