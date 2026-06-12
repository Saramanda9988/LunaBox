import type { ImgHTMLAttributes } from "react";
import { useMemo, useState } from "react";
import { proxiedImageSrc, shouldProxyImageSrc } from "../../utils/imageProxy";

type ProxyImageProps = Omit<ImgHTMLAttributes<HTMLImageElement>, "src"> & {
  src: string | null | undefined;
};

export function ProxyImage({
  src,
  referrerPolicy = "no-referrer",
  draggable = false,
  onDragStart,
  onError,
  ...props
}: ProxyImageProps) {
  const rawSrc = src?.trim() ?? "";
  const proxySrc = useMemo(() => proxiedImageSrc(rawSrc), [rawSrc]);
  const [failedProxySrc, setFailedProxySrc] = useState("");
  const shouldUseOriginalSrc
    = failedProxySrc === proxySrc && shouldProxyImageSrc(rawSrc);
  const resolvedSrc = shouldUseOriginalSrc ? rawSrc : proxySrc;

  return (
    <img
      {...props}
      src={resolvedSrc}
      referrerPolicy={referrerPolicy}
      draggable={draggable}
      onError={(event) => {
        if (!shouldUseOriginalSrc && shouldProxyImageSrc(rawSrc)) {
          setFailedProxySrc(proxySrc);
          return;
        }
        onError?.(event);
      }}
      onDragStart={onDragStart ?? (event => event.preventDefault())}
    />
  );
}
