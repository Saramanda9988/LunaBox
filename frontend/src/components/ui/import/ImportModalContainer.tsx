import type { ReactNode } from "react";
import { ModalPortal } from "../ModalPortal";

interface ImportModalContainerProps {
  title: string;
  iconClassName: string;
  onClose: () => void;
  children: ReactNode;
}

export function ImportModalContainer({
  title,
  iconClassName,
  onClose,
  children,
}: ImportModalContainerProps) {
  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div className="flex max-h-[90vh] w-full max-w-4xl flex-col rounded-xl bg-white shadow-2xl dark:bg-brand-800">
          <div className="flex items-center justify-between border-b border-brand-200 p-6 dark:border-brand-700">
            <div className="flex items-center gap-3">
              <div className={iconClassName} />
              <h2 className="text-2xl font-bold text-brand-900 dark:text-white">
                {title}
              </h2>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="i-mdi-close rounded-lg p-1 text-2xl text-brand-500 hover:bg-brand-100 hover:text-brand-700 focus:outline-none dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
            />
          </div>

          <div className="flex-1 overflow-y-auto p-6">{children}</div>
        </div>
      </div>
    </ModalPortal>
  );
}
