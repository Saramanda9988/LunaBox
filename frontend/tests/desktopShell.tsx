import { StrictMode, useState } from "react";
import { createRoot } from "react-dom/client";
import { BetterDrawer } from "../src/components/ui/better/BetterDrawer";
import { BetterSelect } from "../src/components/ui/better/BetterSelect";
import {
  APP_MODAL_ROOT_ID,
  ModalPortal,
} from "../src/components/ui/ModalPortal";
import "@unocss/reset/tailwind.css";
import "virtual:uno.css";

const TITLEBAR_HEIGHT = 28;

export function Fixture() {
  const [open, setOpen] = useState(false);
  const [nested, setNested] = useState(false);
  const [modal, setModal] = useState(false);
  const [value, setValue] = useState("one");
  const glass = new URLSearchParams(window.location.search).has("glass");

  return (
    <div
      data-testid="root-layout"
      className="relative h-screen w-full overflow-hidden"
      data-glass={glass ? "true" : "false"}
    >
      {/* Match __root.tsx: the scaled background exposes focus-induced scrolling. */}
      {glass && (
        <div
          className="absolute inset-0"
          style={{
            background: "linear-gradient(135deg, #d9cee6, #9cc7c1)",
            filter: "blur(10px)",
            transform: "scale(1.1)",
          }}
        />
      )}
      <div className="relative flex h-full w-full flex-col text-brand-900 dark:text-brand-100">
        <header
          data-testid="chrome"
          className="shrink-0 bg-brand-50 dark:bg-brand-800"
          style={{ height: TITLEBAR_HEIGHT }}
        >
          LunaBox
        </header>
        <div className="relative flex-1 overflow-hidden">
          <div className="absolute left-0 top-0 h-full w-full shrink-0">
            <main data-testid="content" className="h-full overflow-auto p-6">
              <button
                type="button"
                data-testid="open"
                onClick={() => setOpen(true)}
              >
                Open drawer
              </button>
              <button
                type="button"
                data-testid="open-modal"
                onClick={() => setModal(true)}
              >
                Open modal
              </button>
              <BetterDrawer
                isOpen={open}
                onOpenChange={setOpen}
                title="Test drawer"
                topOffset={TITLEBAR_HEIGHT}
              >
                <input aria-label="Test input" />
                <BetterSelect
                  value={value}
                  onChange={setValue}
                  options={[
                    { value: "one", label: "One" },
                    { value: "two", label: "Two" },
                  ]}
                />
                <button
                  type="button"
                  data-testid="nested"
                  onClick={() => setNested(true)}
                >
                  Nested drawer
                </button>
                <BetterDrawer
                  isOpen={nested}
                  onOpenChange={setNested}
                  title="Nested drawer"
                  topOffset={TITLEBAR_HEIGHT}
                >
                  <button type="button">Nested action</button>
                </BetterDrawer>
              </BetterDrawer>
            </main>
            <div
              id={APP_MODAL_ROOT_ID}
              className="absolute inset-0 z-60 pointer-events-none"
            />
            {modal && (
              <ModalPortal>
                <div
                  data-testid="local-modal"
                  className="absolute inset-0 bg-white dark:bg-brand-800"
                >
                  <button type="button" onClick={() => setModal(false)}>
                    Close modal
                  </button>
                </div>
              </ModalPortal>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Fixture />
  </StrictMode>,
);
