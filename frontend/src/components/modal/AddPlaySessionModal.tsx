import { useState } from "react";
import { createPortal } from "react-dom";
import { toast } from "react-hot-toast";
import { AddPlaySession } from "../../../bindings/lunabox/internal/service/TimerService";
import { formatDuration, toLocalISOString } from "../../utils/time";

interface AddPlaySessionModalProps {
  isOpen: boolean;
  gameId: string;
  onClose: () => void;
  onSuccess: () => void;
}

export function AddPlaySessionModal({ isOpen, gameId, onClose, onSuccess }: AddPlaySessionModalProps) {
  const [startTime, setStartTime] = useState(() => {
    const now = new Date();
    now.setHours(now.getHours() - 1);
    return toLocalISOString(now).slice(0, 16);
  });
  const [endTime, setEndTime] = useState(() => {
    const now = new Date();
    return toLocalISOString(now).slice(0, 16);
  });
  const [isSubmitting, setIsSubmitting] = useState(false);

  if (!isOpen)
    return null;

  const calculateDuration = () => {
    if (!startTime || !endTime)
      return 0;
    const start = new Date(startTime);
    const end = new Date(endTime);
    const diffSeconds = Math.floor((end.getTime() - start.getTime()) / 1000);
    return Math.max(0, diffSeconds);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const start = new Date(startTime);
    const end = new Date(endTime);

    if (start >= end) {
      toast.error("结束时间必须晚于开始时间");
      return;
    }

    const totalSeconds = calculateDuration();
    if (totalSeconds <= 0) {
      toast.error("游玩时长必须大于0");
      return;
    }

    if (end > new Date()) {
      toast.error("结束时间不能晚于当前时间");
      return;
    }

    setIsSubmitting(true);
    try {
      const totalMinutes = Math.floor(totalSeconds / 60);
      // 使用本地时间格式（不带 Z 后缀），后端会直接解析为本地时间
      await AddPlaySession(gameId, toLocalISOString(start), totalMinutes);
      toast.success("游玩记录添加成功");
      onSuccess();
      onClose();
      // 重置表单
      const now = new Date();
      const oneHourAgo = new Date(now);
      oneHourAgo.setHours(now.getHours() - 1);
      setStartTime(toLocalISOString(oneHourAgo).slice(0, 16));
      setEndTime(toLocalISOString(now).slice(0, 16));
    }
    catch (error) {
      console.error("Failed to add play session:", error);
      toast.error("添加游玩记录失败");
    }
    finally {
      setIsSubmitting(false);
    }
  };

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="relative bg-white dark:bg-brand-800 rounded-lg shadow-xl w-full max-w-md mx-4 p-6">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-semibold text-brand-900 dark:text-white">
            添加游玩记录
          </h2>
          <button
            onClick={onClose}
            className="text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-white transition-colors"
          >
            <div className="i-mdi-close text-xl" />
          </button>
        </div>

        <div className="text-sm text-brand-600 dark:text-brand-400 mb-4">
          您可以对过去的游玩时间进行补充记录，以更准确地统计总游玩时长。
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              开始时间
            </label>
            <input
              type="datetime-local"
              value={startTime}
              onChange={e => setStartTime(e.target.value)}
              max={toLocalISOString(new Date()).slice(0, 16)}
              className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              结束时间
            </label>
            <input
              type="datetime-local"
              value={endTime}
              onChange={e => setEndTime(e.target.value)}
              max={toLocalISOString(new Date()).slice(0, 16)}
              className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
              required
            />
          </div>

          <div className="bg-brand-50 dark:bg-brand-700/50 rounded-lg p-3">
            <div className="text-sm text-brand-600 dark:text-brand-400 mb-1">游玩时长</div>
            <div className="text-lg font-semibold text-brand-900 dark:text-white">
              {formatDuration(calculateDuration())}
            </div>
          </div>

          <div className="flex justify-end gap-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-brand-600 dark:text-brand-400 hover:bg-brand-100 dark:hover:bg-brand-700 rounded-md transition-colors"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isSubmitting ? "添加中..." : "添加"}
            </button>
          </div>
        </form>
      </div>
    </div>,
    document.body,
  );
}
