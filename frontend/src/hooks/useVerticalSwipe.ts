import type { MouseEventHandler, PointerEventHandler } from "react";

import { useCallback, useRef } from "react";

const SWIPE_DISTANCE_PX = 56;
const SWIPE_AXIS_DOMINANCE = 1.2;

type SwipeState = {
  pointerId: number;
  startClientX: number;
  startClientY: number;
};

type UseVerticalSwipeOptions = {
  enabled?: boolean;
  onSwipeDown: () => void;
  onSwipeUp: () => void;
};

type VerticalSwipeHandlers = {
  onClickCapture: MouseEventHandler<HTMLElement>;
  onPointerCancel: PointerEventHandler<HTMLElement>;
  onPointerDown: PointerEventHandler<HTMLElement>;
  onPointerUp: PointerEventHandler<HTMLElement>;
};

export function useVerticalSwipe({
  enabled = true,
  onSwipeDown,
  onSwipeUp,
}: UseVerticalSwipeOptions): VerticalSwipeHandlers {
  const swipeRef = useRef<SwipeState | null>(null);
  const suppressClickUntilRef = useRef(0);
  const onSwipeDownRef = useRef(onSwipeDown);
  const onSwipeUpRef = useRef(onSwipeUp);
  onSwipeDownRef.current = onSwipeDown;
  onSwipeUpRef.current = onSwipeUp;

  const onPointerDown = useCallback<PointerEventHandler<HTMLElement>>(
    (event) => {
      if (
        !enabled
        || !event.isPrimary
        || (event.pointerType !== "touch" && event.pointerType !== "pen")
      ) {
        return;
      }

      swipeRef.current = {
        pointerId: event.pointerId,
        startClientX: event.clientX,
        startClientY: event.clientY,
      };
    },
    [enabled],
  );

  const onPointerUp = useCallback<PointerEventHandler<HTMLElement>>((event) => {
    const swipe = swipeRef.current;
    swipeRef.current = null;
    if (!swipe || swipe.pointerId !== event.pointerId) {
      return;
    }

    const deltaX = event.clientX - swipe.startClientX;
    const deltaY = event.clientY - swipe.startClientY;
    if (
      Math.abs(deltaY) < SWIPE_DISTANCE_PX
      || Math.abs(deltaY) < Math.abs(deltaX) * SWIPE_AXIS_DOMINANCE
    ) {
      return;
    }

    event.preventDefault();
    suppressClickUntilRef.current = performance.now() + 500;
    if (deltaY < 0) {
      onSwipeUpRef.current();
    }
    else {
      onSwipeDownRef.current();
    }
  }, []);

  const onPointerCancel = useCallback<PointerEventHandler<HTMLElement>>(() => {
    swipeRef.current = null;
  }, []);

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
    onPointerCancel,
    onPointerDown,
    onPointerUp,
  };
}
