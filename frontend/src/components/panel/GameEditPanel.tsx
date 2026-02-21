import type { models } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { OpenLocalPath } from "../../../wailsjs/go/service/GameService";
import { BetterButton } from "../ui/BetterButton";
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
  const { t } = useTranslation();

  return (
    <div className="glass-panel mx-auto bg-white dark:bg-brand-800 p-8 rounded-lg shadow-sm">
      <div className="space-y-6">
        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.name")}
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
            {t("gameEdit.cover")}
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.cover_url}
              onChange={e => onGameChange({ ...game, cover_url: e.target.value } as models.Game)}
              placeholder={t("gameEdit.coverPlaceholder")}
              className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <BetterButton onClick={onSelectCoverImage} icon="i-mdi-image" title={t("gameEdit.selectImage")} />
          </div>
          <p className="mt-1 text-xs text-brand-500">{t("gameEdit.coverHint")}</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.developer")}
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
            {t("gameEdit.path")}
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.path}
              onChange={e => onGameChange({ ...game, path: e.target.value } as models.Game)}
              className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <div className="flex items-center gap-1">
              <BetterButton onClick={onSelectExecutable} icon="i-mdi-file" title={t("gameEdit.selectFile")} />
              <BetterButton
                onClick={async () => {
                  try {
                    await OpenLocalPath(game.path);
                  }
                  catch {
                    toast.error(t("gameEdit.openPathFailed"));
                  }
                }}
                disabled={!game.path}
                icon="i-mdi-folder-open"
                title={t("gameEdit.openInExplorer")}
              />
            </div>
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.savePath")}
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.save_path || ""}
              onChange={e => onGameChange({ ...game, save_path: e.target.value } as models.Game)}
              placeholder={t("gameEdit.savePathPlaceholder")}
              className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <div className="flex items-center gap-1">
              <BetterButton onClick={onSelectSaveDirectory} icon="i-mdi-folder" title={t("gameEdit.selectFolder")} />
              <BetterButton onClick={onSelectSaveFile} icon="i-mdi-file" title={t("gameEdit.selectFile")} />
              <BetterButton
                onClick={async () => {
                  if (!game.save_path)
                    return;
                  try {
                    await OpenLocalPath(game.save_path);
                  }
                  catch {
                    toast.error(t("gameEdit.openPathFailed"));
                  }
                }}
                disabled={!game.save_path}
                icon="i-mdi-folder-open"
                title={t("gameEdit.openInExplorer")}
              />
            </div>
          </div>
          <p className="mt-1 text-xs text-brand-500">{t("gameEdit.savePathHint")}</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.summary")}
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
              {t("gameEdit.sourceType")}
            </label>
            <BetterSelect
              value={game.source_type || ""}
              onChange={value => onGameChange({ ...game, source_type: value } as models.Game)}
              options={[
                { value: "", label: t("gameEdit.sourceNone") },
                { value: "local", label: t("gameEdit.sourceLocal") },
                { value: "bangumi", label: "Bangumi" },
                { value: "vndb", label: "VNDB" },
                { value: "ymgal", label: t("gameEdit.sourceYmgal") },
              ]}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameEdit.sourceId")}
            </label>
            <input
              type="text"
              value={game.source_id || ""}
              onChange={e => onGameChange({ ...game, source_id: e.target.value } as models.Game)}
              placeholder={t("gameEdit.sourceIdPlaceholder")}
              className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
          </div>
        </div>

        <div className="flex justify-between pt-4">
          <div className="flex gap-4 justify-end w-full">
            {onUpdateFromRemote && (
              <BetterButton
                variant="primary"
                onClick={onUpdateFromRemote}
                icon="i-mdi-cloud-sync"
              >
                {t("gameEdit.updateFromRemote")}
              </BetterButton>
            )}
            <BetterButton
              variant="danger"
              onClick={onDelete}
              icon="i-mdi-trash-can-outline"
            >
              {t("common.delete")}
            </BetterButton>
          </div>
        </div>
      </div>
    </div>
  );
}
