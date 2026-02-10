interface CategoryModalProps {
  isOpen: boolean;
  value: string;
  onChange: (value: string) => void;
  onClose: () => void;
  onSubmit: () => void;
  mode?: "add" | "edit";
}

export function CategoryModal({
  isOpen,
  value,
  onChange,
  onClose,
  onSubmit,
  mode = "add",
}: CategoryModalProps) {
  if (!isOpen)
    return null;

  const title = mode === "add" ? "新建收藏夹" : "编辑收藏夹";
  const submitText = mode === "add" ? "创建" : "保存";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800">
        <h3 className="text-xl font-bold text-brand-900 dark:text-white mb-4">{title}</h3>
        <input
          type="text"
          value={value}
          onChange={e => onChange(e.target.value)}
          placeholder="收藏夹名称"
          className="w-full p-2 border border-brand-300 rounded-lg mb-4 dark:bg-brand-700 dark:border-brand-600 dark:text-white focus:ring-2 focus:ring-neutral-500"
          autoFocus
        />
        <div className="flex justify-end gap-2">
          <button
            onClick={onClose}
            className="px-4 py-2 text-brand-700 hover:bg-brand-100 rounded-lg dark:text-brand-300 dark:hover:bg-brand-700"
          >
            取消
          </button>
          <button
            onClick={onSubmit}
            disabled={!value.trim()}
            className="px-4 py-2 bg-neutral-600 text-white rounded-lg hover:bg-neutral-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {submitText}
          </button>
        </div>
      </div>
    </div>
  );
}
