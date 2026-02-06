import type { appconf, models } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { BetterSwitch } from "../ui/BetterSwitch";

interface GameLaunchPanelProps {
  game: models.Game;
  config?: appconf.AppConfig;
  onGameChange: (game: models.Game) => void;
  onSelectProcessExecutable: () => void;
}

export function GameLaunchPanel({ game, config, onGameChange, onSelectProcessExecutable }: GameLaunchPanelProps) {
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
      <div className="space-y-8">
        {/* 进程监控配置 */}
        <div className="flex items-center gap-2">
          <div className="i-mdi-monitor text-xl text-brand-600 dark:text-brand-400" />
          <h3 className="text-sm font-semibold text-brand-900 dark:text-white">进程监控</h3>
        </div>

        <div className="rounded-lg space-y-4">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              可执行程序
            </label>
            <div className="text-sm text-neutral-600 dark:text-neutral-400 font-mono bg-white dark:bg-brand-800 px-3 py-1.5 rounded border border-brand-200 dark:border-brand-700">
              {game.path ? game.path.split(/[\\/]/).pop() : "未设置游戏路径"}
            </div>
            <p className="mt-1 text-xs text-brand-400">
              LunaBox 使用此可执行文件启动游戏。这里是您一开始选择的可执行文件路径
            </p>
          </div>

          <div className="border-t border-brand-200 dark:border-brand-700 my-4" />

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              实际游戏进程
            </label>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={game.process_name || ""}
                onChange={e => onGameChange({ ...game, process_name: e.target.value } as models.Game)}
                placeholder="例如: dota2.exe (留空则自动检测)"
                className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none font-mono"
              />
              <button
                type="button"
                onClick={onSelectProcessExecutable}
                className="glass-btn-neutral px-4 py-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
              >
                选择
              </button>
            </div>
            <p className="mt-2 text-xs text-brand-400 leading-relaxed">
              指定实际游戏的进程名称（包含 .exe 后缀）。
              <br />
              如果不指定，LunaBox 会尝试监控可执行路径。如果之前的选择是启动器，您需要在游戏启动后弹出的选择框中手动指定此进程。
              在此处预先填写可避免每次启动时手动选择。
            </p>
          </div>
        </div>

        {/* 启动工具配置 */}
        <div>
          <div className="flex items-center gap-2 mb-4">
            <div className="i-mdi-tools text-xl text-brand-600 dark:text-brand-400" />
            <h3 className="text-sm font-semibold text-brand-900 dark:text-white">启动增强工具</h3>
          </div>

          <div className="flex items-center justify-between p-4">
            <div className="flex-1 mr-4">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-brand-700 dark:text-brand-300">Locale Emulator</span>
                <span className="px-1.5 py-0.5 text-[10px] font-medium bg-brand-200 dark:bg-brand-600 text-brand-800 dark:text-brand-100 rounded">转区工具</span>
              </div>
              <p className="text-xs text-neutral-500 dark:text-neutral-400 mt-1">
                使用日文环境模拟启动游戏，解决乱码和区域限制问题。
              </p>
              {!hasLocaleEmulatorPath && (
                <p className="text-xs text-error-500 mt-1 flex items-center gap-1">
                  <div className="i-mdi-alert-circle text-sm" />
                  {" "}
                  请先在设置中配置 LEProc.exe 路径
                </p>
              )}
            </div>
            <BetterSwitch
              id="use_locale_emulator"
              checked={game.use_locale_emulator || false}
              onCheckedChange={handleLocaleEmulatorToggle}
              disabled={!hasLocaleEmulatorPath}
            />
          </div>

          <div className="flex items-center justify-between p-4">
            <div className="flex-1 mr-4">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-brand-700 dark:text-brand-300">Magpie</span>
                <span className="px-1.5 py-0.5 text-[10px] font-medium bg-brand-200 dark:bg-brand-600 text-brand-800 dark:text-brand-100 rounded">超分缩放</span>
              </div>
              <p className="text-xs text-neutral-500 dark:text-neutral-400 mt-1">
                游戏启动后自动启动 Magpie 进行全屏超分辨率缩放。
              </p>
              {!hasMagpiePath && (
                <p className="text-xs text-error-500 mt-1 flex items-center gap-1">
                  <div className="i-mdi-alert-circle text-sm" />
                  {" "}
                  请先在设置中配置 Magpie.exe 路径
                </p>
              )}
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
    </div>
  );
}
