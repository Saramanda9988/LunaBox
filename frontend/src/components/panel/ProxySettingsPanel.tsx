import type { appconf } from "../../../src/bindings/models";
import { useTranslation } from "react-i18next";
import { BetterSelect } from "../ui/better/BetterSelect";

interface ProxySettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function ProxySettingsPanel({
  formData,
  onChange,
}: ProxySettingsPanelProps) {
  const { t } = useTranslation();
  const modeOptions = [
    {
      value: "system",
      label: t("settings.proxy.modeSystem", "自动跟随系统代理"),
    },
    { value: "manual", label: t("settings.proxy.modeManual", "使用手动代理") },
    {
      value: "direct",
      label: t("settings.proxy.modeDirect", "直连，不使用代理"),
    },
  ];
  const proxyMode = formData.network_proxy_mode || "system";

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.proxy.globalMode", "全局网络代理")}
        </label>
        <BetterSelect
          value={proxyMode}
          onChange={value =>
            onChange({
              ...formData,
              network_proxy_mode: value,
            } as appconf.AppConfig)}
          options={modeOptions}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t(
            "settings.proxy.globalModeHint",
            "应用内的元数据、图片、游戏下载、AI、WebSearch、应用更新和云同步请求都会使用此代理策略。",
          )}
        </p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.proxy.manualProxyURL", "手动代理 URL")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t(
            "settings.proxy.manualProxyURLHint",
            "仅在选择手动代理时生效。支持 http://、https://、socks5://，也可直接填写 127.0.0.1:7890。",
          )}
        </p>
        <input
          type="text"
          value={formData.network_proxy_url || ""}
          onChange={e =>
            onChange({
              ...formData,
              network_proxy_url: e.target.value,
            } as appconf.AppConfig)}
          placeholder={t(
            "settings.proxy.manualProxyURLPlaceholder",
            "例如 http://127.0.0.1:7890",
          )}
          className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
        />
        {proxyMode !== "manual" && (
          <p className="text-xs text-brand-400 dark:text-brand-500">
            {t(
              "settings.proxy.manualProxyURLIdle",
              "当前未使用手动代理，此地址会保存但暂不生效。",
            )}
          </p>
        )}
      </div>
    </div>
  );
}
