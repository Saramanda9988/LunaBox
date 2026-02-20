import type { models } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { OpenLocalPath } from "../../../wailsjs/go/service/GameService";
import { BetterSelect } from "../ui/BetterSelect";

interface GameEditFormProps {
  game: models.Game;
  onGameChange: (game: models.Game) => void;
  onDelete: () => void;
  onSelectExecutable: () => void;
  onSelectSaveDirectory: () => void;
  onSelectSaveFile: () => void;
  onSelectCoverImage: () => void;
  onUpdateFromRemote?: () => void;
}

export function GameEditPanel({
  game,
  onGameChange,
  onDelete,
  onSelectExecutable,
  onSelectSaveDirectory,
  onSelectSaveFile,
  onSelectCoverImage,
  onUpdateFromRemote,
}: GameEditFormProps) {
  return (
    <div className="glass-panel mx-auto bg-white dark:bg-brand-800 p-8 rounded-lg shadow-sm">
      <div className="space-y-6">
        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            游戏名称
          </label>
          <input
            type="text"
            value={game.name}
            onChange={e => onGameChange({ ...game, name: e.target.value } as models.Game)}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            封面图片
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.cover_url}
              onChange={e => onGameChange({ ...game, cover_url: e.target.value } as models.Game)}
              placeholder="输入图片 URL 或选择本地图片"
              className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <button
              type="button"
              onClick={onSelectCoverImage}
              className="glass-btn-neutral px-4 py-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
            >
              <div className="i-mdi-file text-xl" />
            </button>
          </div>
          <p className="mt-1 text-xs text-brand-500">支持远端url获取和本地图片选取</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            开发商
          </label>
          <input
            type="text"
            value={game.company}
            onChange={e => onGameChange({ ...game, company: e.target.value } as models.Game)}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            游戏路径
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.path}
              onChange={e => onGameChange({ ...game, path: e.target.value } as models.Game)}
              className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={onSelectExecutable}
                title="选择文件"
                className="glass-btn-neutral px-4 py-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
              >
                <div className="i-mdi-file text-xl" />
              </button>
              <button
                type="button"
                onClick={async () => {
                  try {
                    await OpenLocalPath(game.path);
                  }
                  catch {
                    toast.error("打开路径失败，文件/目录可能不存在");
                  }
                }}
                disabled={!game.path}
                title="在文件管理器中打开位置"
                className="glass-btn-neutral p-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <div className="i-mdi-folder-open text-xl" />
              </button>
            </div>
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            存档路径
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.save_path || ""}
              onChange={e => onGameChange({ ...game, save_path: e.target.value } as models.Game)}
              placeholder="选择游戏存档路径（文件或文件夹）"
              className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={onSelectSaveDirectory}
                title="选择文件夹"
                className="glass-btn-neutral p-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
              >
                <div className="i-mdi-folder text-xl" />
              </button>
              <button
                type="button"
                onClick={onSelectSaveFile}
                title="选择文件"
                className="glass-btn-neutral p-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
              >
                <div className="i-mdi-file text-xl" />
              </button>
              <button
                type="button"
                onClick={async () => {
                  if (!game.save_path)
                    return;
                  try {
                    await OpenLocalPath(game.save_path);
                  }
                  catch {
                    toast.error("打开路径失败，文件/目录可能不存在");
                  }
                }}
                disabled={!game.save_path}
                title="在文件管理器中打开位置"
                className="glass-btn-neutral p-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <div className="i-mdi-folder-open text-xl" />
              </button>
            </div>
          </div>
          <p className="mt-1 text-xs text-brand-500">设置存档路径（文件或文件夹）后可使用备份功能</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            简介
          </label>
          <textarea
            value={game.summary}
            onChange={e => onGameChange({ ...game, summary: e.target.value } as models.Game)}
            rows={6}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none resize-none"
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              数据源类型
            </label>
            <BetterSelect
              value={game.source_type || ""}
              onChange={value => onGameChange({ ...game, source_type: value } as models.Game)}
              options={[
                { value: "", label: "无" },
                { value: "local", label: "本地" },
                { value: "bangumi", label: "Bangumi" },
                { value: "vndb", label: "VNDB" },
                { value: "ymgal", label: "月幕Galgame" },
              ]}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              数据源ID
            </label>
            <input
              type="text"
              value={game.source_id || ""}
              onChange={e => onGameChange({ ...game, source_id: e.target.value } as models.Game)}
              placeholder="远程数据源的ID"
              className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
          </div>
        </div>

        <div className="flex justify-between pt-4">
          <div className="flex gap-4 justify-end w-full">
            {onUpdateFromRemote && (
              <button
                type="button"
                onClick={onUpdateFromRemote}
                className="glass-btn-neutral px-6 py-2 bg-accent-500 text-white rounded-md hover:bg-accent-700 transition-colors focus:ring-2 focus:ring-offset-2 focus:ring-accent-500"
              >
                从远程更新
              </button>
            )}
            <button
              type="button"
              onClick={onDelete}
              className="glass-btn-error px-6 py-2 bg-error-500 text-white rounded-md hover:bg-error-700 transition-colors focus:ring-2 focus:ring-offset-2 focus:ring-error-500"
            >
              删除
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
