import type { ImgHTMLAttributes } from "react";
import { useMemo, useState } from "react";
import { useAppStore } from "../../store";
import { imageSourceCandidates } from "../../utils/imageProxy";

type ProxyImageProps = Omit<ImgHTMLAttributes<HTMLImageElement>, "src"> & {
  src: string | null | undefined;
  fallbackSrc?: string | null | undefined;
  isNSFW?: boolean;
  revealNSFWOnHover?: boolean;
};

export function ProxyImage({
  src,
  fallbackSrc,
  referrerPolicy = "no-referrer",
  draggable = false,
  onDragStart,
  onError,
  className,
  isNSFW = false,
  revealNSFWOnHover = false,
  ...props
}: ProxyImageProps) {
  const shouldBlurNSFW = useAppStore(
    state => state.config?.blur_nsfw_game_covers !== false,
  );
  const rawSrc = src?.trim() ?? "";
  const rawFallbackSrc = fallbackSrc?.trim() ?? "";
  const candidates = useMemo(
    () => imageSourceCandidates(rawSrc, rawFallbackSrc),
    [rawFallbackSrc, rawSrc],
  );
  const candidateSignature = candidates.join("\0");
  const [failureState, setFailureState] = useState({
    signature: candidateSignature,
    index: 0,
  });
  const failureIndex
    = failureState.signature === candidateSignature ? failureState.index : 0;
  const resolvedSrc = candidates[failureIndex] ?? "";

  return (
    <img
      {...props}
      src={resolvedSrc}
      className={`${className ?? ""} ${
        isNSFW && shouldBlurNSFW
          ? `nsfw-cover-blur will-change-[filter] transition-[filter] duration-300 ${
            revealNSFWOnHover ? "hover:nsfw-cover-reveal" : ""
          }`
          : ""
      }`.trim()}
      referrerPolicy={referrerPolicy}
      draggable={draggable}
      onError={(event) => {
        if (failureIndex + 1 < candidates.length) {
          setFailureState({
            signature: candidateSignature,
            index: failureIndex + 1,
          });
          return;
        }
        onError?.(event);
      }}
      onDragStart={onDragStart ?? (event => event.preventDefault())}
    />
  );
}
