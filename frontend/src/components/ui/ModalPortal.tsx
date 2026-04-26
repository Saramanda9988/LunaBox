import type { ReactNode } from "react";
import { createPortal } from "react-dom";

export const APP_MODAL_ROOT_ID = "app-modal-root";

interface ModalPortalProps {
  children: ReactNode;
}

export function ModalPortal({ children }: ModalPortalProps) {
  const target = document.getElementById(APP_MODAL_ROOT_ID) ?? document.body;

  return createPortal(
    <div className="absolute inset-0 pointer-events-auto">{children}</div>,
    target,
  );
}
