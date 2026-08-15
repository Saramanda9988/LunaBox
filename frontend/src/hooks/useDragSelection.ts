import type { MouseEventHandler, PointerEventHandler, RefObject } from "react";

import { useCallback, useEffect, useRef } from "react";

import { findScrollParent } from "../utils/scroll";

const DRAG_SELECTION_ATTRIBUTE = "data-drag-selection-id";
const DRAG_START_DISTANCE_PX = 10;
const AUTO_SCROLL_EDGE_PX = 64;
const AUTO_SCROLL_MAX_STEP_PX = 18;

type DragGestureState = {
  active: boolean;
  desiredSelection: boolean;
  lastClientX: number;
  lastClientY: number;
  pointerId: number;
  pointerType: string;
  scrollElement: HTMLElement | null;
  startClientX: number;
  startClientY: number;
  startId: string;
  visitedIds: Set<string>;
};

type UseDragSelectionOptions = {
  enabled: boolean;
  onSelectChange?: (id: string, selected: boolean) => void;
  selectedIds?: ReadonlySet<string>;
  surfaceRef: RefObject<HTMLElement | null>;
};

export type DragSelectionHandlers = {
  onClickCapture: MouseEventHandler<HTMLElement>;
  onContextMenu: MouseEventHandler<HTMLElement>;
  onPointerCancel: PointerEventHandler<HTMLElement>;
  onPointerDown: PointerEventHandler<HTMLElement>;
  onPointerMove: PointerEventHandler<HTMLElement>;
  onPointerUp: PointerEventHandler<HTMLElement>;
};

function findSelectionId(
  surface: HTMLElement | null,
  target: EventTarget | null,
) {
  if (!surface || !(target instanceof Element)) {
    return null;
  }

  const selectable = target.closest<HTMLElement>(
    `[${DRAG_SELECTION_ATTRIBUTE}]`,
  );
  if (!selectable || !surface.contains(selectable)) {
    return null;
  }

  return selectable.getAttribute(DRAG_SELECTION_ATTRIBUTE);
}

export function useDragSelection({
  enabled,
  onSelectChange,
  selectedIds,
  surfaceRef,
}: UseDragSelectionOptions): DragSelectionHandlers {
  const gestureRef = useRef<DragGestureState | null>(null);
  const animationFrameRef = useRef<number | null>(null);
  const suppressClickUntilRef = useRef(0);
  const onSelectChangeRef = useRef(onSelectChange);
  const selectedIdsRef = useRef(selectedIds);
  onSelectChangeRef.current = onSelectChange;
  selectedIdsRef.current = selectedIds;

  const selectAtPoint = useCallback(
    (clientX: number, clientY: number, gesture: DragGestureState) => {
      const target = document.elementFromPoint(clientX, clientY);
      const id = findSelectionId(surfaceRef.current, target);
      if (!id || gesture.visitedIds.has(id)) {
        return;
      }

      gesture.visitedIds.add(id);
      onSelectChangeRef.current?.(id, gesture.desiredSelection);
    },
    [surfaceRef],
  );

  const stopAutoScroll = useCallback(() => {
    if (animationFrameRef.current !== null) {
      window.cancelAnimationFrame(animationFrameRef.current);
      animationFrameRef.current = null;
    }
  }, []);

  const runAutoScroll = useCallback(() => {
    const gesture = gestureRef.current;
    const scrollElement = gesture?.scrollElement;
    if (!gesture?.active || !scrollElement) {
      animationFrameRef.current = null;
      return;
    }

    const rect = scrollElement.getBoundingClientRect();
    const topDistance = gesture.lastClientY - rect.top;
    const bottomDistance = rect.bottom - gesture.lastClientY;
    let step = 0;

    if (topDistance >= 0 && topDistance < AUTO_SCROLL_EDGE_PX) {
      step = -AUTO_SCROLL_MAX_STEP_PX * (1 - topDistance / AUTO_SCROLL_EDGE_PX);
    }
    else if (bottomDistance >= 0 && bottomDistance < AUTO_SCROLL_EDGE_PX) {
      step
        = AUTO_SCROLL_MAX_STEP_PX * (1 - bottomDistance / AUTO_SCROLL_EDGE_PX);
    }

    if (step === 0) {
      animationFrameRef.current = null;
      return;
    }

    const previousScrollTop = scrollElement.scrollTop;
    scrollElement.scrollTop += step;
    if (scrollElement.scrollTop !== previousScrollTop) {
      selectAtPoint(gesture.lastClientX, gesture.lastClientY, gesture);
    }
    animationFrameRef.current = window.requestAnimationFrame(runAutoScroll);
  }, [selectAtPoint]);

  const updateAutoScroll = useCallback(() => {
    if (animationFrameRef.current === null) {
      animationFrameRef.current = window.requestAnimationFrame(runAutoScroll);
    }
  }, [runAutoScroll]);

  const finishGesture = useCallback(() => {
    stopAutoScroll();
    gestureRef.current = null;
  }, [stopAutoScroll]);

  useEffect(() => finishGesture, [finishGesture]);
  useEffect(() => {
    if (!enabled) {
      finishGesture();
    }
  }, [enabled, finishGesture]);

  const onPointerDown = useCallback<PointerEventHandler<HTMLElement>>(
    (event) => {
      if (!enabled || !event.isPrimary) {
        return;
      }

      const isTouchPointer
        = event.pointerType === "touch" || event.pointerType === "pen";
      const isRightMouseButton
        = event.pointerType === "mouse" && event.button === 2;
      if (!isTouchPointer && !isRightMouseButton) {
        return;
      }

      const startId = findSelectionId(surfaceRef.current, event.target);
      if (!startId) {
        return;
      }

      const gesture: DragGestureState = {
        active: isRightMouseButton,
        desiredSelection: !selectedIdsRef.current?.has(startId),
        lastClientX: event.clientX,
        lastClientY: event.clientY,
        pointerId: event.pointerId,
        pointerType: event.pointerType,
        scrollElement: findScrollParent(surfaceRef.current),
        startClientX: event.clientX,
        startClientY: event.clientY,
        startId,
        visitedIds: new Set<string>(),
      };
      gestureRef.current = gesture;

      if (isRightMouseButton) {
        event.preventDefault();
        event.currentTarget.setPointerCapture(event.pointerId);
        gesture.visitedIds.add(startId);
        onSelectChangeRef.current?.(startId, gesture.desiredSelection);
      }
    },
    [enabled, surfaceRef],
  );

  const onPointerMove = useCallback<PointerEventHandler<HTMLElement>>(
    (event) => {
      const gesture = gestureRef.current;
      if (!gesture || gesture.pointerId !== event.pointerId) {
        return;
      }

      gesture.lastClientX = event.clientX;
      gesture.lastClientY = event.clientY;

      if (!gesture.active) {
        const distance = Math.hypot(
          event.clientX - gesture.startClientX,
          event.clientY - gesture.startClientY,
        );
        if (distance < DRAG_START_DISTANCE_PX) {
          return;
        }

        gesture.active = true;
        event.currentTarget.setPointerCapture(event.pointerId);
        gesture.visitedIds.add(gesture.startId);
        onSelectChangeRef.current?.(gesture.startId, gesture.desiredSelection);
      }

      event.preventDefault();
      selectAtPoint(event.clientX, event.clientY, gesture);
      updateAutoScroll();
    },
    [selectAtPoint, updateAutoScroll],
  );

  const onPointerUp = useCallback<PointerEventHandler<HTMLElement>>(
    (event) => {
      const gesture = gestureRef.current;
      if (!gesture || gesture.pointerId !== event.pointerId) {
        return;
      }

      if (gesture.active) {
        event.preventDefault();
        if (gesture.pointerType !== "mouse") {
          suppressClickUntilRef.current = performance.now() + 500;
        }
      }
      if (event.currentTarget.hasPointerCapture(event.pointerId)) {
        event.currentTarget.releasePointerCapture(event.pointerId);
      }
      finishGesture();
    },
    [finishGesture],
  );

  const onPointerCancel = useCallback<PointerEventHandler<HTMLElement>>(
    () => finishGesture(),
    [finishGesture],
  );

  const onContextMenu = useCallback<MouseEventHandler<HTMLElement>>(
    (event) => {
      if (enabled && findSelectionId(surfaceRef.current, event.target)) {
        event.preventDefault();
      }
    },
    [enabled, surfaceRef],
  );

  const onClickCapture = useCallback<MouseEventHandler<HTMLElement>>(
    (event) => {
      if (performance.now() > suppressClickUntilRef.current) {
        return;
      }
      suppressClickUntilRef.current = 0;
      event.preventDefault();
      event.stopPropagation();
    },
    [],
  );

  return {
    onClickCapture,
    onContextMenu,
    onPointerCancel,
    onPointerDown,
    onPointerMove,
    onPointerUp,
  };
}
