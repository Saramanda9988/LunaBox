import { createRootRoute, Outlet } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";
import { SideBar } from "../components/bar/SideBar";
import { TopBar } from "../components/bar/TopBar";
import { DragDropImportModal } from "../components/modal/DragDropImportModal";
import { useAppStore } from "../store";

function RootLayout() {
  const { t } = useTranslation();
  const { config, fetchGames } = useAppStore();
  const [isDragOver, setIsDragOver] = useState(false);
  const [showDragDropModal, setShowDragDropModal] = useState(false);
  const [droppedPaths, setDroppedPaths] = useState<string[]>([]);

  const bgEnabled = config?.background_enabled && config?.background_image;
  const bgBlur = config?.background_blur ?? 10;
  const bgOpacity = config?.background_opacity ?? 0.85;

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

      {/* Main content container */}
      <div className="relative flex h-full w-full flex-col text-brand-900 dark:text-brand-100">
        <TopBar />

        <div className="flex flex-1 overflow-hidden">
          <SideBar bgEnabled={!!bgEnabled} bgOpacity={bgOpacity} />
          <main
            className={`flex-1 overflow-auto ${bgEnabled ? "" : "bg-brand-100 dark:bg-brand-900"
            }`}
            style={bgEnabled ? {
              backgroundColor: `rgba(var(--main-bg-rgb), ${bgOpacity})`,
            } : undefined}
          >
            <Outlet />
          </main>
        </div>
      </div>

      {/* Drag overlay */}
      {isDragOver && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-primary-500/20 backdrop-blur-sm pointer-events-none">
          <div className="flex flex-col items-center gap-4 p-8 rounded-2xl bg-white/90 dark:bg-brand-800/90 shadow-2xl border-2 border-dashed border-primary-500">
            <div className="i-mdi-folder-upload text-6xl text-primary-500 animate-bounce" />
            <div className="text-center">
              <p className="text-xl font-bold text-brand-900 dark:text-white">
                {t("root.dragDrop.dropToImport")}
              </p>
              <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">
                {t("root.dragDrop.dropHint")}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Drag-drop import modal */}
      <DragDropImportModal
        isOpen={showDragDropModal}
        droppedPaths={droppedPaths}
        onClose={handleCloseDragDropModal}
        onImportComplete={handleImportComplete}
      />
    </div>
  );
}

export const Route = createRootRoute({
  component: RootLayout,
});
