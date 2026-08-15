import { createRootRoute, Outlet } from "@tanstack/react-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { onWailsEvent } from "../../src/bindings/runtime";
import { invalidateAllGameLists } from "../cache/gameCache";
import { PlayingIsland } from "../components/bar/PlayingIsland";
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
  const fetchHomeData = useAppStore(state => state.fetchHomeData);
  const [isDragOver, setIsDragOver] = useState(false);
  const [showDragDropModal, setShowDragDropModal] = useState(false);
  const [droppedPaths, setDroppedPaths] = useState<string[]>([]);
  const dragLeaveTimerRef = useRef<number | null>(null);
  const hasFileDragRef = useRef(false);
  const isDropImportActiveRef = useRef(false);

  const bgEnabled = config?.background_enabled && config?.background_image;
  const bgBlur = config?.background_blur ?? 10;
  const bgOpacity = config?.background_opacity ?? 0.85;
  const zoomFactor = normalizeAppZoomFactor(config?.window_zoom_factor);

  useEffect(() => {
    return onWailsEvent<string[]>("files-dropped", (paths) => {
      if (dragLeaveTimerRef.current !== null) {
        window.clearTimeout(dragLeaveTimerRef.current);
        dragLeaveTimerRef.current = null;
      }
      hasFileDragRef.current = false;
      setIsDragOver(false);
      if (!paths || paths.length === 0 || isDropImportActiveRef.current) {
        return;
      }

      isDropImportActiveRef.current = true;
      setDroppedPaths(paths);
      setShowDragDropModal(true);
    });
  }, []);

  useEffect(() => {
    const resetDragOverlay = () => {
      if (dragLeaveTimerRef.current !== null) {
        window.clearTimeout(dragLeaveTimerRef.current);
        dragLeaveTimerRef.current = null;
      }
      hasFileDragRef.current = false;
      setIsDragOver(false);
    };

    const isFileDrag = (event: DragEvent) =>
      event.dataTransfer?.types.includes("Files") ?? false;

    const handleDragEnterOrOver = (event: DragEvent) => {
      if (!isFileDrag(event)) {
        return;
      }

      event.preventDefault();
      if (event.dataTransfer) {
        event.dataTransfer.dropEffect = "copy";
      }
      hasFileDragRef.current = true;
      if (dragLeaveTimerRef.current !== null) {
        window.clearTimeout(dragLeaveTimerRef.current);
        dragLeaveTimerRef.current = null;
      }
      if (!isDropImportActiveRef.current) {
        setIsDragOver(true);
      }
    };

    const handleDragLeave = (event: DragEvent) => {
      if (!hasFileDragRef.current) {
        return;
      }

      event.preventDefault();
      if (dragLeaveTimerRef.current !== null) {
        window.clearTimeout(dragLeaveTimerRef.current);
      }
      dragLeaveTimerRef.current = window.setTimeout(resetDragOverlay, 80);
    };

    const handleDrop = (event: DragEvent) => {
      if (!isFileDrag(event) && !hasFileDragRef.current) {
        return;
      }

      // Always suppress WebView2's default file navigation. Wails' own
      // listener still receives the event and resolves native file paths.
      event.preventDefault();
      resetDragOverlay();
    };

    window.addEventListener("dragenter", handleDragEnterOrOver);
    window.addEventListener("dragover", handleDragEnterOrOver);
    window.addEventListener("dragleave", handleDragLeave);
    window.addEventListener("drop", handleDrop);
    window.addEventListener("blur", resetDragOverlay);

    return () => {
      resetDragOverlay();
      window.removeEventListener("dragenter", handleDragEnterOrOver);
      window.removeEventListener("dragover", handleDragEnterOrOver);
      window.removeEventListener("dragleave", handleDragLeave);
      window.removeEventListener("drop", handleDrop);
      window.removeEventListener("blur", resetDragOverlay);
    };
  }, []);

  const handleImportComplete = useCallback(() => {
    invalidateAllGameLists();
    void fetchHomeData({ showLoading: false, syncRuntime: false });
  }, [fetchHomeData]);

  const handleCloseDragDropModal = useCallback(() => {
    isDropImportActiveRef.current = false;
    setShowDragDropModal(false);
    setDroppedPaths([]);
  }, []);

  return (
    <div
      className="relative h-screen w-full overflow-hidden"
      data-glass={bgEnabled ? "true" : "false"}
      data-file-drop-target
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
        <PlayingIsland />
        <AppToaster topOffset={(TOPBAR_HEIGHT + 12) * zoomFactor} />

        <div className="relative flex-1 overflow-hidden">
          <div className="absolute left-0 top-0 h-full w-full shrink-0">
            <div className="flex h-full w-full overflow-hidden">
              <SideBar bgEnabled={!!bgEnabled} bgOpacity={bgOpacity} />
              <main
                className={`@container flex-1 overflow-auto ${
                  bgEnabled ? "" : "bg-brand-100 dark:bg-brand-900"
                }`}
                style={{
                  containerType: "inline-size",
                  ...(bgEnabled && {
                    backgroundColor: `rgba(var(--main-bg-rgb), ${bgOpacity})`,
                  }),
                }}
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
