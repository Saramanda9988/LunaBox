import { createRootRoute, Outlet } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";
import { SideBar } from "../components/bar/SideBar";
import { TopBar, TOPBAR_HEIGHT } from "../components/bar/TopBar";
import { DragDropImportModal } from "../components/modal/DragDropImportModal";
import { AppToaster } from "../components/ui/AppToaster";
import { APP_MODAL_ROOT_ID } from "../components/ui/ModalPortal";
import { normalizeAppZoomFactor } from "../consts/options";
import { useAppStore } from "../store";

function RootLayout() {
  const { t } = useTranslation();
  const config = useAppStore(state => state.config);
  const fetchGames = useAppStore(state => state.fetchGames);
  const [isDragOver, setIsDragOver] = useState(false);
  const [showDragDropModal, setShowDragDropModal] = useState(false);
  const [droppedPaths, setDroppedPaths] = useState<string[]>([]);

  const bgEnabled = config?.background_enabled && config?.background_image;
  const bgBlur = config?.background_blur ?? 10;
  const bgOpacity = config?.background_opacity ?? 0.85;
  const zoomFactor = normalizeAppZoomFactor(config?.window_zoom_factor);
  const zoomStyle
    = zoomFactor === 1
      ? undefined
      : ({
          height: `${100 / zoomFactor}%`,
          transform: `scale(${zoomFactor})`,
          transformOrigin: "top left",
          width: `${100 / zoomFactor}%`,
        } satisfies React.CSSProperties);

  useEffect(() => {
    OnFileDrop((_x: number, _y: number, paths: string[]) => {
      setIsDragOver(false);
      if (paths && paths.length > 0) {
        setDroppedPaths(paths);
        setShowDragDropModal(true);
      }
    }, true);

    return () => {
      OnFileDropOff();
    };
  }, []);

  useEffect(() => {
    const handleDragOver = (e: DragEvent) => {
      e.preventDefault();
      const target = e.target as HTMLElement;
      if (target.tagName === "IMG") {
        return;
      }
      if (e.dataTransfer?.types.includes("Files")) {
        setIsDragOver(true);
      }
    };

    const handleDragLeave = (e: DragEvent) => {
      if (e.relatedTarget === null) {
        setIsDragOver(false);
      }
    };

    const handleDrop = (e: DragEvent) => {
      const target = e.target as HTMLElement;
      if (target.tagName === "IMG") {
        e.preventDefault();
        e.stopPropagation();
        return;
      }
      setIsDragOver(false);
    };

    window.addEventListener("dragover", handleDragOver);
    window.addEventListener("dragleave", handleDragLeave);
    window.addEventListener("drop", handleDrop);

    return () => {
      window.removeEventListener("dragover", handleDragOver);
      window.removeEventListener("dragleave", handleDragLeave);
      window.removeEventListener("drop", handleDrop);
    };
  }, []);

  const handleImportComplete = () => {
    fetchGames();
  };

  const handleCloseDragDropModal = () => {
    setShowDragDropModal(false);
    setDroppedPaths([]);
  };

  return (
    <div
      className="relative h-screen w-full overflow-hidden"
      data-glass={bgEnabled ? "true" : "false"}
      style={{ "--wails-drop-target": "drop" } as React.CSSProperties}
    >
      {/* Background layer */}
      {bgEnabled && (
        <div
          key={`bg-${bgBlur}-${config.background_image}`}
          className="absolute inset-0 bg-cover bg-center bg-no-repeat transition-all duration-300"
          style={{
            backgroundImage: `url("${config.background_image}")`,
            filter: `blur(${bgBlur}px)`,
            transform: "scale(1.1)",
          }}
        />
      )}

      <div className="relative flex h-full w-full flex-col text-brand-900 dark:text-brand-100">
        <TopBar />
        <AppToaster topOffset={TOPBAR_HEIGHT + 12} />

        <div className="relative flex-1 overflow-hidden">
          <div
            className="absolute left-0 top-0 h-full w-full shrink-0"
            style={zoomStyle}
          >
            <div className="flex h-full w-full overflow-hidden">
              <SideBar bgEnabled={!!bgEnabled} bgOpacity={bgOpacity} />
              <main
                className={`flex-1 overflow-auto ${
                  bgEnabled ? "" : "bg-brand-100 dark:bg-brand-900"
                }`}
                style={
                  bgEnabled
                    ? {
                        backgroundColor: `rgba(var(--main-bg-rgb), ${bgOpacity})`,
                      }
                    : undefined
                }
              >
                <Outlet />
              </main>
            </div>

            {/* Drag overlay */}
            {isDragOver && (
              <div className="absolute inset-0 z-50 flex items-center justify-center bg-primary-500/20 backdrop-blur-sm pointer-events-none">
                <div className="flex flex-col items-center gap-4 rounded-2xl border-2 border-dashed border-primary-500 bg-white/90 p-8 shadow-2xl dark:bg-brand-800/90">
                  <div className="i-mdi-folder-upload animate-bounce text-6xl text-primary-500" />
                  <div className="text-center">
                    <p className="text-xl font-bold text-brand-900 dark:text-white">
                      {t("root.dragDrop.dropToImport")}
                    </p>
                    <p className="mt-1 text-sm text-brand-500 dark:text-brand-400">
                      {t("root.dragDrop.dropHint")}
                    </p>
                  </div>
                </div>
              </div>
            )}

            <div
              id={APP_MODAL_ROOT_ID}
              className="absolute inset-0 z-60 pointer-events-none"
            />

            {/* Drag-drop import modal */}
            <DragDropImportModal
              isOpen={showDragDropModal}
              droppedPaths={droppedPaths}
              onClose={handleCloseDragDropModal}
              onImportComplete={handleImportComplete}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

export const Route = createRootRoute({
  component: RootLayout,
});
