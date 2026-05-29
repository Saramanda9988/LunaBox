import type { ReactNode } from "react";

export interface BetterDataTableColumn<T> {
  key: string;
  header: ReactNode;
  className?: string;
  headerClassName?: string;
  cellClassName?: string;
  render: (row: T, index: number) => ReactNode;
}

interface BetterDataTableProps<T> {
  rows: T[];
  columns: BetterDataTableColumn<T>[];
  rowKey: (row: T, index: number) => string;
  empty?: ReactNode;
  maxHeightClassName?: string;
  rowClassName?: (row: T, index: number) => string;
}

export function BetterDataTable<T>({
  rows,
  columns,
  rowKey,
  empty,
  maxHeightClassName = "max-h-[400px]",
  rowClassName,
}: BetterDataTableProps<T>) {
  return (
    <div
      className={[
        maxHeightClassName,
        "overflow-auto rounded-lg border border-brand-200",
        "bg-white/80 dark:border-brand-700 dark:bg-brand-900/50",
        "data-glass:bg-white/10 data-glass:dark:bg-black/10",
      ].join(" ")}
    >
      {rows.length === 0 ? (
        <div className="p-8 text-center text-sm text-brand-400 dark:text-brand-500">
          {empty}
        </div>
      ) : (
        <table className="w-full table-fixed border-separate border-spacing-0">
          <thead className="sticky top-0 z-10 bg-brand-50/95 backdrop-blur dark:bg-brand-800/95 data-glass:bg-white/20 data-glass:dark:bg-black/30">
            <tr>
              {columns.map(column => (
                <th
                  key={column.key}
                  className={[
                    "border-b border-brand-200 px-3 py-2 text-left text-xs font-semibold",
                    "text-brand-600 dark:border-brand-700 dark:text-brand-300",
                    column.className,
                    column.headerClassName,
                  ]
                    .filter(Boolean)
                    .join(" ")}
                >
                  {column.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-brand-100 dark:divide-brand-800">
            {rows.map((row, index) => (
              <tr
                key={rowKey(row, index)}
                className={[
                  "transition-colors hover:bg-brand-50/80 dark:hover:bg-brand-800/70",
                  rowClassName?.(row, index),
                ]
                  .filter(Boolean)
                  .join(" ")}
              >
                {columns.map(column => (
                  <td
                    key={column.key}
                    className={[
                      "px-3 py-2 align-middle text-sm text-brand-700 dark:text-brand-300",
                      column.className,
                      column.cellClassName,
                    ]
                      .filter(Boolean)
                      .join(" ")}
                  >
                    {column.render(row, index)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
