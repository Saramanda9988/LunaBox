import type { ButtonHTMLAttributes } from "react";

type BetterEdgeIconButtonPlacement = "bottom" | "left" | "right";

interface BetterEdgeIconButtonProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "children"
> {
  icon: string;
  iconClassName?: string;
  placement?: BetterEdgeIconButtonPlacement;
}

const placementClasses: Record<BetterEdgeIconButtonPlacement, string> = {
  bottom: "h-10 w-16 rounded-t-xl",
  left: "h-16 w-10 rounded-r-xl",
  right: "h-16 w-10 rounded-l-xl",
};

const iconSizeClasses: Record<BetterEdgeIconButtonPlacement, string> = {
  bottom: "text-2xl",
  left: "text-3xl",
  right: "text-3xl",
};

export function BetterEdgeIconButton({
  icon,
  iconClassName = "",
  placement = "bottom",
  className = "",
  type = "button",
  ...rest
}: BetterEdgeIconButtonProps) {
  return (
    <button
      type={type}
      className={[
        "glass-btn-neutral flex shrink-0 items-center justify-center",
        "border border-white/30 bg-white/40 text-brand-700 opacity-75 shadow-lg backdrop-blur-md",
        "transition-all duration-200 hover:bg-white/65 hover:opacity-100 active:scale-95",
        "hover:border-white/30 focus:border-white/30 focus:outline-none",
        "disabled:cursor-not-allowed disabled:opacity-40 disabled:active:scale-100",
        "dark:border-white/15 dark:bg-black/35 dark:text-brand-200 dark:hover:border-white/15 dark:hover:bg-black/55 dark:focus:border-white/15",
        placementClasses[placement],
        className,
      ].join(" ")}
      {...rest}
    >
      <span
        className={`${icon} ${iconSizeClasses[placement]} ${iconClassName}`}
        aria-hidden="true"
      />
    </button>
  );
}
