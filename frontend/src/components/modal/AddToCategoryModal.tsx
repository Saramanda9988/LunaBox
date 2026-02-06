import type { vo } from "../../../wailsjs/go/models";
import { useEffect, useState } from "react";

interface AddToCategoryModalProps {
  isOpen: boolean;
  allCategories: vo.CategoryVO[];
  initialSelectedIds: string[];
  onClose: () => void;
  onSave: (selectedIds: string[]) => void;
}

export function AddToCategoryModal({
  isOpen,
  allCategories,
  initialSelectedIds,
  onClose,
  onSave,
}: AddToCategoryModalProps) {
  const [selectedIds, setSelectedIds] = useState<string[]>(initialSelectedIds);

  useEffect(() => {
    setSelectedIds(initialSelectedIds);
  }, [initialSelectedIds]);

  if (!isOpen)
    return null;

  const toggleCategory = (categoryId: string) => {
    setSelectedIds(prev =>
      prev.includes(categoryId)
        ? prev.filter(id => id !== categoryId)
        : [...prev, categoryId],
    );
  };

  const handleSave = () => {
    onSave(selectedIds);
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="w-full max-w-md max-h-[70vh] rounded-xl bg-white flex flex-col shadow-xl dark:bg-brand-800">
        <div className="p-6 border-b border-brand-200 dark:border-brand-700 flex justify-between items-center">
          <h3 className="text-xl font-bold text-brand-900 dark:text-white">添加到收藏</h3>
          <button
            onClick={onClose}
            className="text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-white"
          >
            <div className="i-mdi-close text-xl" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          {allCategories.length > 0
            ? (
                <div className="space-y-2">
                  {allCategories.map((category) => {
                    const isSelected = selectedIds.includes(category.id);
                    return (
                      <button
                        key={category.id}
                        onClick={() => toggleCategory(category.id)}
                        className={`w-full flex items-center justify-between p-3 rounded-lg transition-colors ${
                          isSelected
                            ? "bg-neutral-100 dark:bg-neutral-900"
                            : "bg-brand-50 dark:bg-brand-900 hover:bg-brand-100 dark:hover:bg-brand-700"
                        }`}
                      >
                        <span className="font-medium text-brand-900 dark:text-white">
                          {category.name}
                        </span>
                        <div className="flex items-center gap-2 text-sm text-brand-500 dark:text-brand-400">
                          <span>
                            {category.game_count || 0}
                            {" "}
                            个游戏
                          </span>
                          {isSelected
                            ? (
                                <div className="i-mdi-check-circle text-neutral-600 dark:text-neutral-400 text-xl" />
                              )
                            : (
                                <div className="i-mdi-circle-outline text-brand-300 dark:text-brand-600 text-xl" />
                              )}
                        </div>
                      </button>
                    );
                  })}
                </div>
              )
            : (
                <div className="flex flex-col items-center justify-center h-full text-brand-500">
                  <div className="i-mdi-folder-outline text-4xl mb-2" />
                  <p>暂无收藏夹</p>
                </div>
              )}
        </div>

        <div className="p-4 border-brand-200 dark:border-brand-700 flex gap-3">
          <button
            onClick={onClose}
            className="flex-1 py-2 border border-brand-300 text-brand-600 rounded-lg hover:bg-brand-50 dark:border-brand-600 dark:text-brand-400 dark:hover:bg-brand-700 font-medium"
          >
            取消
          </button>
          <button
            onClick={handleSave}
            className="flex-1 py-2 bg-neutral-600 text-white rounded-lg hover:bg-neutral-700 dark:bg-neutral-600 dark:hover:bg-neutral-700 font-medium"
          >
            确定
          </button>
        </div>
      </div>
    </div>
  );
}
