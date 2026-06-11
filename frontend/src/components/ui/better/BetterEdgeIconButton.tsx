import type { ButtonHTMLAttributes } from "react";

type BetterEdgeIconButtonPlacement = "bottom" | "left" | "right";

interface BetterEdgeIconButtonProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "children"
> {
  blurClassName?: string;
  icon: string;
  iconClassName?: string;
  placement?: BetterEdgeIconButtonPlacement;
  surfaceClassName?: string;
}

const placementClasses: Record<BetterEdgeIconButtonPlacement, string> = {
  bottom: "h-8 w-12 rounded-t-lg",
  left: "h-16 w-10 rounded-r-xl",
  right: "h-16 w-10 rounded-l-xl",
};

const iconSizeClasses: Record<BetterEdgeIconButtonPlacement, string> = {
  bottom: "text-xl",
  left: "text-3xl",
  right: "text-3xl",
};

export function BetterEdgeIconButton({
  blurClassName = "backdrop-blur-md data-glass:backdrop-blur-12",
  icon,
  iconClassName = "",
  placement = "bottom",
  className = "",
  surfaceClassName = "bg-white/40 hover:bg-white/65 dark:bg-black/35 dark:hover:bg-black/55 data-glass:bg-white/30 data-glass:hover:bg-white/40 data-glass:dark:bg-black/30 data-glass:dark:hover:bg-black/40",
  type = "button",
  ...rest
}: BetterEdgeIconButtonProps) {
  return (
    <button
      type={type}
      className={[
        "flex shrink-0 items-center justify-center",
        "border border-white/30 text-brand-700 opacity-75 shadow-lg",
        surfaceClassName,
        blurClassName,
        "transition-all duration-200 hover:opacity-100 active:scale-95",
        "hover:border-white/30 focus:border-white/30 focus:outline-none",
        "disabled:cursor-not-allowed disabled:opacity-40 disabled:active:scale-100",
        "dark:border-white/15 dark:text-brand-200 dark:hover:border-white/15 dark:focus:border-white/15",
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
