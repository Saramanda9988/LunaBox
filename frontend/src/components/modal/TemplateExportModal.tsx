import type {
  PeriodStats,
  TemplateInfo,
} from "../../../bindings/lunabox/internal/vo";
import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import toast from "react-hot-toast";
import {
  ExportRenderedHTML,
  ListTemplates,
  OpenTemplatesDir,
  PrepareExportData,
  RenderTemplate,
} from "../../../bindings/lunabox/internal/service/templateservice";
import {
  RenderTemplateRequest,
} from "../../../bindings/lunabox/internal/vo";

interface TemplateExportModalProps {
  isOpen: boolean;
  onClose: () => void;
  stats: PeriodStats | null;
  aiSummary: string;
}

export function TemplateExportModal({
  isOpen,
  onClose,
  stats,
  aiSummary,
}: TemplateExportModalProps) {
  const [templates, setTemplates] = useState<TemplateInfo[]>([]);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>("");
  const [previewHtml, setPreviewHtml] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const previewRef = useRef<HTMLDivElement>(null);

  const loadTemplates = async () => {
    try {
      const list = await ListTemplates();
      setTemplates(list);
      if (list.length > 0 && !selectedTemplateId) {
        setSelectedTemplateId(list[0].id);
      }
    }
    catch (err) {
      console.error("Failed to load templates:", err);
      toast.error("加载模板列表失败");
    }
  };

  const renderPreview = async () => {
    if (!stats || !selectedTemplateId)
      return;

    setLoading(true);
    try {
      // 准备导出数据（包含图表数据，由后端处理）
      const exportData = await PrepareExportData(stats, aiSummary);

      // 渲染模板
      const req = new RenderTemplateRequest({
        template_id: selectedTemplateId,
        data: exportData,
      });
      const resp = await RenderTemplate(req);
      setPreviewHtml(resp.html);
    }
    catch (err) {
      console.error("Failed to render template:", err);
      toast.error("渲染模板失败");
    }
    finally {
      setLoading(false);
    }
  };

  const handleExport = useCallback(async () => {
    if (!previewRef.current)
      return;

    setExporting(true);
    try {
      // 获取 iframe
      const iframe = previewRef.current.querySelector("iframe") as HTMLIFrameElement;
      if (!iframe || !iframe.contentWindow) {
        throw new Error("无法获取预览内容");
      }

      const iframeDoc = iframe.contentDocument || iframe.contentWindow.document;
      if (!iframeDoc || !iframeDoc.body) {
        throw new Error("无法获取预览内容");
      }

      // 等待字体加载完成
      try {
        await (iframeDoc as Document & { fonts?: FontFaceSet }).fonts?.ready;
      }
      catch {
        // Fallback: wait a bit for fonts to load
        await new Promise(resolve => setTimeout(resolve, 1000));
      }

      // 动态导入 html2canvas
      const html2canvas = (await import("html2canvas")).default;

      // 在 iframe 内注入 html2canvas 并执行截图
      const iframeWindow = iframe.contentWindow as Window & { html2canvas?: typeof html2canvas };

      // 注入 html2canvas 到 iframe
      if (!iframeWindow.html2canvas) {
        const script = iframeDoc.createElement("script");
        script.src = "https://cdn.jsdelivr.net/npm/html2canvas@1.4.1/dist/html2canvas.min.js";
        iframeDoc.head.appendChild(script);
        await new Promise<void>((resolve, reject) => {
          script.onload = () => resolve();
          script.onerror = () => reject(new Error("Failed to load html2canvas"));
          setTimeout(() => resolve(), 3000); // Timeout fallback
        });
      }

      // 使用 iframe 内的 html2canvas 或外部的
      const h2c = iframeWindow.html2canvas || html2canvas;
      const targetElement = iframeDoc.body;

      // 重置滚动位置，避免偏移
      iframeWindow.scrollTo(0, 0);
      iframeDoc.documentElement.scrollTop = 0;
      iframeDoc.body.scrollTop = 0;

      // 等待一帧确保滚动位置已重置
      await new Promise(resolve => requestAnimationFrame(resolve));

      const canvas = await h2c(targetElement, {
        backgroundColor: null,
        scale: 2,
        useCORS: true,
        allowTaint: true,
        logging: false,
        x: 0,
        y: 0,
        scrollX: 0,
        scrollY: 0,
        windowWidth: targetElement.scrollWidth,
        windowHeight: targetElement.scrollHeight,
        onclone: (clonedDoc: Document) => {
          // 重置克隆文档的滚动位置
          clonedDoc.documentElement.scrollTop = 0;
          clonedDoc.body.scrollTop = 0;
          // 确保克隆的文档使用正确的字体
          const style = clonedDoc.createElement("style");
          style.textContent = `
            * {
              -webkit-font-smoothing: antialiased;
              -moz-osx-font-smoothing: grayscale;
            }
            html, body {
              margin: 0 !important;
              padding: 0 !important;
            }
          `;
          clonedDoc.head.appendChild(style);
        },
      });

      const dataUrl = canvas.toDataURL("image/png");

      // 保存图片
      await ExportRenderedHTML(dataUrl);
      onClose();
    }
    catch (err) {
      console.error("Failed to export image:", err);
      toast.error(`导出图片失败: ${err instanceof Error ? err.message : String(err)}`);
    }
    finally {
      setExporting(false);
    }
  }, [previewRef, onClose]);

  const handleOpenTemplatesDir = async () => {
    try {
      await OpenTemplatesDir();
    }
    catch (err) {
      console.error("Failed to open templates dir:", err);
      toast.error("打开模板目录失败");
    }
  };

  // 加载模板列表
  useEffect(() => {
    if (isOpen) {
      loadTemplates();
    }
  }, [isOpen]);

  // 当选择模板或数据变化时，渲染预览
  useEffect(() => {
    if (isOpen && selectedTemplateId && stats) {
      renderPreview();
    }
  }, [isOpen, selectedTemplateId, stats, aiSummary]);

  if (!isOpen)
    return null;

  const selectedTemplate = templates.find(t => t.id === selectedTemplateId);

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="w-full max-w-6xl h-[85vh] rounded-xl bg-white shadow-xl dark:bg-brand-800 border border-brand-200 dark:border-brand-700 flex flex-col overflow-hidden">
        {/* 头部 */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-brand-200 dark:border-brand-700">
          <div className="flex items-center gap-3">
            <span className="i-mdi-image-filter-hdr text-2xl text-neutral-600 dark:text-neutral-400" />
            <h2 className="text-xl font-bold text-brand-900 dark:text-white">美化导出</h2>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 transition-colors"
          >
            <span className="i-mdi-close text-xl text-brand-600 dark:text-brand-400" />
          </button>
        </div>

        {/* 主体 */}
        <div className="flex-1 flex overflow-hidden">
          {/* 左侧：模板选择 */}
          <div className="w-64 border-r border-brand-200 dark:border-brand-700 flex flex-col">
            <div className="p-4 border-b border-brand-200 dark:border-brand-700">
              <h3 className="text-sm font-semibold text-brand-900 dark:text-white mb-1">选择模板</h3>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                选择一个模板来美化你的统计数据
              </p>
            </div>
            <div className="flex-1 overflow-y-auto p-2 space-y-1">
              {templates.map(template => (
                <button
                  key={template.id}
                  onClick={() => setSelectedTemplateId(template.id)}
                  className={`w-full text-left px-3 py-2.5 rounded-lg transition-colors ${
                    selectedTemplateId === template.id
                      ? "bg-neutral-100 dark:bg-neutral-900 text-neutral-700 dark:text-neutral-300"
                      : "hover:bg-brand-50 dark:hover:bg-brand-700/50 text-brand-700 dark:text-brand-300"
                  }`}
                >
                  <div className="flex items-center gap-2">
                    {template.is_builtin
                      ? (
                          <span className="i-mdi-check-decagram text-neutral-500" />
                        )
                      : (
                          <span className="i-mdi-file-document-outline text-brand-400" />
                        )}
                    <span className="font-medium text-sm">{template.name}</span>
                  </div>
                  {template.description && (
                    <p className="text-xs text-brand-500 dark:text-brand-400 mt-1 ml-6 line-clamp-2">
                      {template.description}
                    </p>
                  )}
                </button>
              ))}
            </div>
            <div className="p-3 border-t border-brand-200 dark:border-brand-700">
              <button
                onClick={handleOpenTemplatesDir}
                className="w-full flex items-center justify-center gap-2 px-3 py-2 text-sm text-brand-600 dark:text-brand-400 hover:bg-brand-50 dark:hover:bg-brand-700/50 rounded-lg transition-colors"
              >
                <span className="i-mdi-folder-open" />
                打开模板目录
              </button>
            </div>
          </div>

          {/* 右侧：预览 */}
          <div className="flex-1 flex flex-col overflow-hidden bg-brand-50 dark:bg-brand-900/50">
            {/* 模板信息 */}
            {selectedTemplate && (
              <div className="px-4 py-3 border-b border-brand-200 dark:border-brand-700 bg-white dark:bg-brand-800">
                <div className="flex items-center justify-between">
                  <div>
                    <h4 className="font-semibold text-brand-900 dark:text-white">
                      {selectedTemplate.name}
                    </h4>
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      {selectedTemplate.author && `作者: ${selectedTemplate.author} · `}
                      版本
                      {" "}
                      {selectedTemplate.version}
                      {selectedTemplate.is_builtin && " · 内置模板"}
                    </p>
                  </div>
                </div>
              </div>
            )}

            {/* 预览区域 */}
            <div ref={previewRef} className="flex-1 overflow-auto p-4">
              {loading
                ? (
                    <div className="flex items-center justify-center h-full">
                      <div className="flex items-center gap-3 text-brand-500 dark:text-brand-400">
                        <span className="i-mdi-loading animate-spin text-2xl" />
                        <span>正在渲染预览...</span>
                      </div>
                    </div>
                  )
                : previewHtml
                  ? (
                      <iframe
                        srcDoc={previewHtml}
                        className="w-full h-full border-0 rounded-lg shadow-lg bg-white"
                        title="模板预览"
                        sandbox="allow-same-origin allow-scripts"
                      />
                    )
                  : (
                      <div className="flex items-center justify-center h-full text-brand-500 dark:text-brand-400">
                        选择模板以预览效果
                      </div>
                    )}
            </div>
          </div>
        </div>

        {/* 底部按钮 */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-brand-200 dark:border-brand-700 bg-brand-50 dark:bg-brand-900/30">
          <p className="text-xs text-brand-500 dark:text-brand-400">
            <span className="i-mdi-information-outline mr-1" />
            提示：你可以在模板目录中创建自定义 HTML 模板
          </p>
          <div className="flex gap-3">
            <button
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-brand-700 hover:bg-brand-100 rounded-lg dark:text-brand-300 dark:hover:bg-brand-700 transition-colors"
            >
              取消
            </button>
            <button
              onClick={handleExport}
              disabled={!previewHtml || exporting}
              className="px-4 py-2 text-sm font-medium text-white bg-neutral-600 hover:bg-neutral-700 rounded-lg shadow-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            >
              {exporting
                ? (
                    <>
                      <span className="i-mdi-loading animate-spin" />
                      导出中...
                    </>
                  )
                : (
                    <>
                      <span className="i-mdi-download" />
                      导出图片
                    </>
                  )}
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
