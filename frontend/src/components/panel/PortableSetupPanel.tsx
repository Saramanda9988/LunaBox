import type { service } from "../../../wailsjs/go/models";
import { useCallback, useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import {
  GetStatus,
  RegisterCLIPath,
  RegisterProtocol,
  UnregisterCLIPath,
  UnregisterProtocol,
} from "../../../wailsjs/go/service/PortableSetupService";
import { BetterButton } from "../ui/better/BetterButton";

type ActionKey
  = | "registerProtocol"
    | "unregisterProtocol"
    | "registerCli"
    | "unregisterCli";

export function PortableSetupPanel() {
  const { t } = useTranslation();
  const [status, setStatus] = useState<service.PortableSetupStatus | null>(
    null,
  );
  const [busy, setBusy] = useState<ActionKey | null>(null);
  const [loading, setLoading] = useState(true);

  const loadStatus = useCallback(async () => {
    try {
      const next = await GetStatus();
      setStatus(next);
    }
    catch (err: any) {
      toast.error(
        t("settings.portableSetup.toast.loadFailed", { error: String(err) }),
      );
    }
  }, [t]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      await loadStatus();
      if (!cancelled) {
        setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [loadStatus]);

  const run = async (
    key: ActionKey,
    action: () => Promise<service.PortableSetupStatus>,
    successMsg: string,
    failureMsg: string,
  ) => {
    if (busy) {
      return;
    }
    setBusy(key);
    try {
      const next = await action();
      setStatus(next);
      toast.success(successMsg);
    }
    catch (err: any) {
      toast.error(t(failureMsg, { error: String(err) }));
    }
    finally {
      setBusy(null);
    }
  };

  if (loading || !status) {
    return (
      <div className="flex items-center gap-2 text-sm text-brand-500 dark:text-brand-400">
        <span className="i-mdi-loading animate-spin text-base" />
        {t("settings.portableSetup.loading")}
      </div>
    );
  }

  const protocolBadge = describeProtocolStatus(status.protocol, t);
  const cliBadge = describeCLIStatus(status.cli, t);
  const isMacOS = status.platform === "darwin";

  const handleProtocolRegister = () =>
    run(
      "registerProtocol",
      RegisterProtocol,
      t("settings.portableSetup.toast.protocolRegistered"),
      "settings.portableSetup.toast.protocolRegisterFailed",
    );
  const handleProtocolUnregister = () =>
    run(
      "unregisterProtocol",
      UnregisterProtocol,
      t("settings.portableSetup.toast.protocolUnregistered"),
      "settings.portableSetup.toast.protocolUnregisterFailed",
    );
  const handleCliRegister = () =>
    run(
      "registerCli",
      RegisterCLIPath,
      t("settings.portableSetup.toast.cliRegistered"),
      "settings.portableSetup.toast.cliRegisterFailed",
    );
  const handleCliUnregister = () =>
    run(
      "unregisterCli",
      UnregisterCLIPath,
      t("settings.portableSetup.toast.cliUnregistered"),
      "settings.portableSetup.toast.cliUnregisterFailed",
    );

  return (
    <div className="space-y-6">
      <p className="text-xs text-brand-500 dark:text-brand-400">
        {t(
          isMacOS
            ? "settings.portableSetup.descriptionMac"
            : "settings.portableSetup.description",
        )}
      </p>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.portableSetup.protocolTitle")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t(
            isMacOS
              ? "settings.portableSetup.protocolHintMac"
              : "settings.portableSetup.protocolHint",
          )}
        </p>
        <StatusLine
          label={t("settings.portableSetup.statusLabel")}
          value={protocolBadge.text}
          tone={protocolBadge.tone}
        />
        {status.protocol.registered && status.protocol.registeredPath && (
          <DetailLine
            label={t("settings.portableSetup.registeredPathLabel")}
            value={status.protocol.registeredPath}
          />
        )}
        <DetailLine
          label={t("settings.portableSetup.currentPathLabel")}
          value={status.executablePath || status.protocol.currentPath || "-"}
        />
        {!isMacOS && (
          <div className="flex flex-wrap gap-2 pt-1">
            <BetterButton
              type="button"
              variant="primary"
              icon="i-mdi-link-variant"
              isLoading={busy === "registerProtocol"}
              onClick={handleProtocolRegister}
            >
              {status.protocol.registered && !status.protocol.upToDate
                ? t("settings.portableSetup.reregisterProtocol")
                : t("settings.portableSetup.registerProtocol")}
            </BetterButton>
            {status.protocol.registered && (
              <BetterButton
                type="button"
                variant="secondary"
                icon="i-mdi-link-variant-off"
                isLoading={busy === "unregisterProtocol"}
                onClick={handleProtocolUnregister}
              >
                {t("settings.portableSetup.unregisterProtocol")}
              </BetterButton>
            )}
          </div>
        )}
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.portableSetup.cliTitle")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t(
            isMacOS
              ? "settings.portableSetup.cliHintMac"
              : "settings.portableSetup.cliHint",
          )}
        </p>
        <StatusLine
          label={t("settings.portableSetup.statusLabel")}
          value={cliBadge.text}
          tone={cliBadge.tone}
        />
        <DetailLine
          label={t("settings.portableSetup.cliPathLabel")}
          value={status.cli.cliPath || "-"}
        />
        {status.cli.installPath && (
          <DetailLine
            label={t("settings.portableSetup.cliInstallPathLabel")}
            value={status.cli.installPath}
          />
        )}
        {!status.cli.available && (
          <p className="text-xs text-amber-600 dark:text-amber-400">
            {t(
              isMacOS
                ? "settings.portableSetup.cliMissingHintMac"
                : "settings.portableSetup.cliMissingHint",
            )}
          </p>
        )}
        <div className="flex flex-wrap gap-2 pt-1">
          <BetterButton
            type="button"
            variant="primary"
            icon="i-mdi-console-line"
            isLoading={busy === "registerCli"}
            disabled={!status.cli.available}
            onClick={handleCliRegister}
          >
            {status.cli.registered
              ? t("settings.portableSetup.reregisterCli")
              : t("settings.portableSetup.registerCli")}
          </BetterButton>
          {status.cli.registered && (
            <BetterButton
              type="button"
              variant="secondary"
              icon="i-mdi-console-line"
              isLoading={busy === "unregisterCli"}
              onClick={handleCliUnregister}
            >
              {t("settings.portableSetup.unregisterCli")}
            </BetterButton>
          )}
        </div>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t(
            isMacOS
              ? "settings.portableSetup.pathReopenHintMac"
              : "settings.portableSetup.pathReopenHint",
          )}
        </p>
      </div>
    </div>
  );
}

type Tone = "ok" | "warn" | "off";

function describeProtocolStatus(
  protocol: service.PortableProtocolStatus,
  t: (key: string) => string,
): { text: string; tone: Tone } {
  if (!protocol.registered) {
    return {
      text: t("settings.portableSetup.status.notRegistered"),
      tone: "off",
    };
  }
  if (!protocol.upToDate) {
    return { text: t("settings.portableSetup.status.stalePath"), tone: "warn" };
  }
  return { text: t("settings.portableSetup.status.registered"), tone: "ok" };
}

function describeCLIStatus(
  cli: service.PortableCLIStatus,
  t: (key: string) => string,
): { text: string; tone: Tone } {
  if (!cli.available) {
    return {
      text: t("settings.portableSetup.status.cliMissing"),
      tone: "warn",
    };
  }
  if (cli.registered) {
    return { text: t("settings.portableSetup.status.registered"), tone: "ok" };
  }
  return {
    text: t("settings.portableSetup.status.notRegistered"),
    tone: "off",
  };
}

function StatusLine({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: Tone;
}) {
  const toneCls
    = tone === "ok"
      ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300"
      : tone === "warn"
        ? "bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300"
        : "bg-brand-100 text-brand-600 dark:bg-brand-700 dark:text-brand-300";
  return (
    <div className="flex flex-wrap items-center gap-2 text-xs">
      <span className="text-brand-500 dark:text-brand-400">{label}</span>
      <span className={`rounded-full px-2 py-0.5 font-medium ${toneCls}`}>
        {value}
      </span>
    </div>
  );
}

function DetailLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-wrap items-baseline gap-2 text-xs">
      <span className="text-brand-500 dark:text-brand-400">{label}</span>
      <code className="break-all rounded bg-brand-100 px-1.5 py-0.5 text-brand-800 dark:bg-brand-700 dark:text-brand-200">
        {value}
      </code>
    </div>
  );
}
