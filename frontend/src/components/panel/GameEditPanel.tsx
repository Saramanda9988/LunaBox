import type { appconf, models } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { BetterSelect } from "../ui/BetterSelect";
import { BetterSwitch } from "../ui/BetterSwitch";

interface GameEditFormProps {
  game: models.Game;
  config?: appconf.AppConfig;
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
  config,
  onGameChange,
  onDelete,
  onSelectExecutable,
  onSelectSaveDirectory,
  onSelectSaveFile,
  onSelectCoverImage,
  onUpdateFromRemote,
}: GameEditFormProps) {
  const hasLocaleEmulatorPath = config?.locale_emulator_path && config?.locale_emulator_path.length > 0;
  const hasMagpiePath = config?.magpie_path && config?.magpie_path.length > 0;

  const handleLocaleEmulatorToggle = (checked: boolean) => {
    if (checked && !hasLocaleEmulatorPath) {
      toast.error("请先在设置中配置 Locale Emulator 路径");
      return;
    }
    onGameChange({ ...game, use_locale_emulator: checked } as models.Game);
  };

  const handleMagpieToggle = (checked: boolean) => {
    if (checked && !hasMagpiePath) {
      toast.error("请先在设置中配置 Magpie 路径");
      return;
    }
    onGameChange({ ...game, use_magpie: checked } as models.Game);
  };
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
              选择
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
            <button
              type="button"
              onClick={onSelectExecutable}
              className="glass-btn-neutral px-4 py-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
            >
              选择
            </button>
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

        {/* 启动工具配置 */}
        <div className="border-t border-brand-200 dark:border-brand-700 pt-6">
          <h3 className="text-sm font-semibold text-brand-900 dark:text-white mb-4">启动工具</h3>

          <div className="space-y-4">
            <div className="glass-card flex items-center justify-between p-3 bg-brand-50 dark:bg-brand-700/30 rounded-lg">
              <div className="flex-1">
                <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                  使用 Locale Emulator 转区启动
                </label>
                <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
                  {hasLocaleEmulatorPath
                    ? "启用后将使用 Locale Emulator 转区启动此游戏"
                    : "请先在设置中配置 Locale Emulator 路径"}
                </p>
              </div>
              <BetterSwitch
                id="use_locale_emulator"
                checked={game.use_locale_emulator || false}
                onCheckedChange={handleLocaleEmulatorToggle}
                disabled={!hasLocaleEmulatorPath}
              />
            </div>

            <div className="glass-card flex items-center justify-between p-3 bg-brand-50 dark:bg-brand-700/30 rounded-lg">
              <div className="flex-1">
                <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                  使用 Magpie 超分辨率缩放
                </label>
                <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
                  {hasMagpiePath
                    ? "启用后将在游戏启动后自动启动 Magpie"
                    : "请先在设置中配置 Magpie 路径"}
                </p>
              </div>
              <BetterSwitch
                id="use_magpie"
                checked={game.use_magpie || false}
                onCheckedChange={handleMagpieToggle}
                disabled={!hasMagpiePath}
              />
            </div>
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
