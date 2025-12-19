import { useState } from 'react'
import toast from 'react-hot-toast'
import { SelectJSONFile, PreviewPlayniteImport, ImportFromPlaynite } from '../../../wailsjs/go/service/ImportService'
import { service } from '../../../wailsjs/go/models'

interface PlayniteImportModalProps {
    isOpen: boolean
    onClose: () => void
    onImportComplete: () => void
}

type Step = 'select' | 'preview' | 'importing' | 'result'

export function PlayniteImportModal({ isOpen, onClose, onImportComplete }: PlayniteImportModalProps) {
    const [step, setStep] = useState<Step>('select')
    const [jsonPath, setJsonPath] = useState('')
    const [previewGames, setPreviewGames] = useState<service.PreviewGame[]>([])
    const [importResult, setImportResult] = useState<service.ImportResult | null>(null)
    const [isLoading, setIsLoading] = useState(false)
    const [skipNoPath, setSkipNoPath] = useState(true)

    if (!isOpen) return null

    const handleSelectFile = async () => {
        try {
            const path = await SelectJSONFile()
            if (path) {
                setJsonPath(path)
                setIsLoading(true)
                try {
                    const games = await PreviewPlayniteImport(path)
                    setPreviewGames(games || [])
                    setStep('preview')
                } catch (error) {
                    console.error('Failed to preview import:', error)
                    toast.error('预览导入内容失败')
                } finally {
                    setIsLoading(false)
                }
            }
        } catch (error) {
            console.error('Failed to select file:', error)
            toast.error('选择文件失败')
        }
    }

    const handleImport = async () => {
        if (!jsonPath) return

        setStep('importing')
        setIsLoading(true)

        try {
            const result = await ImportFromPlaynite(jsonPath, skipNoPath)
            setImportResult(result)
            setStep('result')

            if (result.success > 0) {
                toast.success(`成功导入 ${result.success} 个游戏`)
                onImportComplete()
            }
        } catch (error) {
            console.error('Failed to import:', error)
            toast.error('导入失败')
            setStep('preview')
        } finally {
            setIsLoading(false)
        }
    }

    const resetAndClose = () => {
        setStep('select')
        setJsonPath('')
        setPreviewGames([])
        setImportResult(null)
        setSkipNoPath(true)
        onClose()
    }

    const newGamesCount = previewGames.filter(g => !g.exists && (skipNoPath ? g.has_path : true)).length
    const existingGamesCount = previewGames.filter(g => g.exists).length
    const noPathGamesCount = previewGames.filter(g => !g.has_path && !g.exists).length

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
            <div className="w-full max-w-3xl max-h-[90vh] rounded-xl bg-white shadow-2xl dark:bg-brand-800 flex flex-col">
                {/* Header */}
                <div className="flex items-center justify-between p-6 border-b border-brand-200 dark:border-brand-700">
                    <div className="flex items-center gap-3">
                        <div className="i-mdi-application-import text-3xl text-purple-500" />
                        <h2 className="text-2xl font-bold text-brand-900 dark:text-white">
                            从 Playnite 导入
                        </h2>
                    </div>
                    <button
                        onClick={resetAndClose}
                        className="i-mdi-close text-2xl text-brand-500 p-1 rounded-lg
              hover:bg-brand-100 hover:text-brand-700 focus:outline-none
              dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
                    />
                </div>

                {/* Content */}
                <div className="flex-1 overflow-y-auto p-6">
                    {/* Step: Select File */}
                    {step === 'select' && (
                        <div className="space-y-6">
                            <div className="text-center py-8">
                                <div className="i-mdi-file-document text-6xl text-brand-400 mx-auto mb-4" />
                                <p className="text-brand-600 dark:text-brand-300 mb-2">
                                    选择 Playnite 导出的 JSON 文件
                                </p>
                                <p className="text-sm text-brand-400 dark:text-brand-500">
                                    支持通过 Playnite 导出脚本生成的游戏数据文件
                                </p>
                            </div>

                            <button
                                onClick={handleSelectFile}
                                disabled={isLoading}
                                className="flex w-full items-center justify-center rounded-lg bg-purple-500 py-4 text-white transition hover:bg-purple-600 disabled:opacity-50"
                            >
                                {isLoading ? (
                                    <>
                                        <div className="i-mdi-loading animate-spin mr-2 text-xl" />
                                        加载中...
                                    </>
                                ) : (
                                    <>
                                        <div className="i-mdi-file-find mr-2 text-xl" />
                                        选择 JSON 文件
                                    </>
                                )}
                            </button>
                        </div>
                    )}

                    {/* Step: Preview */}
                    {step === 'preview' && (
                        <div className="space-y-4">
                            {/* Summary */}
                            <div className="flex gap-4">
                                <div className="flex-1 rounded-lg bg-green-50 dark:bg-green-900/20 p-4 text-center">
                                    <div className="text-3xl font-bold text-green-600 dark:text-green-400">
                                        {newGamesCount}
                                    </div>
                                    <div className="text-sm text-green-700 dark:text-green-300">
                                        将导入
                                    </div>
                                </div>
                                <div className="flex-1 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 p-4 text-center">
                                    <div className="text-3xl font-bold text-yellow-600 dark:text-yellow-400">
                                        {existingGamesCount}
                                    </div>
                                    <div className="text-sm text-yellow-700 dark:text-yellow-300">
                                        已存在
                                    </div>
                                </div>
                                {noPathGamesCount > 0 && (
                                    <div className="flex-1 rounded-lg bg-orange-50 dark:bg-orange-900/20 p-4 text-center">
                                        <div className="text-3xl font-bold text-orange-600 dark:text-orange-400">
                                            {noPathGamesCount}
                                        </div>
                                        <div className="text-sm text-orange-700 dark:text-orange-300">
                                            无路径
                                        </div>
                                    </div>
                                )}
                            </div>

                            {/* Skip no-path option */}
                            {noPathGamesCount > 0 && (
                                <div className="rounded-lg bg-orange-50 dark:bg-orange-900/20 p-4">
                                    <label className="flex items-start cursor-pointer">
                                        <input
                                            type="checkbox"
                                            checked={skipNoPath}
                                            onChange={(e) => setSkipNoPath(e.target.checked)}
                                            className="mt-1 mr-3"
                                        />
                                        <div>
                                            <div className="text-sm font-medium text-orange-700 dark:text-orange-300">
                                                跳过无路径的游戏
                                            </div>
                                            <div className="text-xs text-orange-600 dark:text-orange-400 mt-1">
                                                有 {noPathGamesCount} 个游戏没有本地路径，这些可能是网络游戏或已删除的游戏。
                                                建议跳过这些游戏。
                                            </div>
                                        </div>
                                    </label>
                                </div>
                            )}

                            {/* Game List */}
                            <div className="max-h-[300px] overflow-y-auto rounded-lg border border-brand-200 dark:border-brand-700">
                                {previewGames.length === 0 ? (
                                    <div className="p-8 text-center text-brand-400">
                                        未找到游戏数据
                                    </div>
                                ) : (
                                    <table className="w-full">
                                        <thead className="sticky top-0 bg-brand-50 dark:bg-brand-700">
                                            <tr>
                                                <th className="px-4 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300">
                                                    游戏名称
                                                </th>
                                                <th className="px-4 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300">
                                                    开发商
                                                </th>
                                                <th className="px-4 py-2 text-center text-sm font-medium text-brand-600 dark:text-brand-300">
                                                    状态
                                                </th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-brand-100 dark:divide-brand-700">
                                            {previewGames.map((game, index) => {
                                                const willBeSkipped = game.exists || (skipNoPath && !game.has_path)
                                                return (
                                                    <tr
                                                        key={index}
                                                        className={`${willBeSkipped
                                                                ? 'opacity-50'
                                                                : 'hover:bg-brand-50 dark:hover:bg-brand-750'
                                                            }`}
                                                    >
                                                        <td className="px-4 py-3 text-sm text-brand-900 dark:text-white">
                                                            {game.name}
                                                            {!game.has_path && (
                                                                <span className="ml-2 text-xs text-orange-500">
                                                                    (无路径)
                                                                </span>
                                                            )}
                                                        </td>
                                                        <td className="px-4 py-3 text-sm text-brand-500 dark:text-brand-400">
                                                            {game.developer || '-'}
                                                        </td>
                                                        <td className="px-4 py-3 text-center">
                                                            {game.exists ? (
                                                                <span className="inline-flex items-center rounded-full bg-yellow-100 px-2 py-1 text-xs text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400">
                                                                    <div className="i-mdi-check-circle mr-1" />
                                                                    已存在
                                                                </span>
                                                            ) : !game.has_path && skipNoPath ? (
                                                                <span className="inline-flex items-center rounded-full bg-orange-100 px-2 py-1 text-xs text-orange-700 dark:bg-orange-900/30 dark:text-orange-400">
                                                                    <div className="i-mdi-close-circle mr-1" />
                                                                    将跳过
                                                                </span>
                                                            ) : (
                                                                <span className="inline-flex items-center rounded-full bg-green-100 px-2 py-1 text-xs text-green-700 dark:bg-green-900/30 dark:text-green-400">
                                                                    <div className="i-mdi-plus-circle mr-1" />
                                                                    新增
                                                                </span>
                                                            )}
                                                        </td>
                                                    </tr>
                                                )
                                            })}
                                        </tbody>
                                    </table>
                                )}
                            </div>

                            {/* Actions */}
                            <div className="flex justify-between">
                                <button
                                    onClick={() => setStep('select')}
                                    className="rounded-lg border border-brand-300 px-5 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-100 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700"
                                >
                                    ← 重新选择
                                </button>
                                <button
                                    onClick={handleImport}
                                    disabled={newGamesCount === 0}
                                    className="rounded-lg bg-purple-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-purple-700 disabled:opacity-50"
                                >
                                    导入 {newGamesCount} 个游戏
                                </button>
                            </div>
                        </div>
                    )}

                    {/* Step: Importing */}
                    {step === 'importing' && (
                        <div className="py-12 text-center">
                            <div className="i-mdi-loading animate-spin text-5xl text-purple-500 mx-auto mb-4" />
                            <p className="text-lg text-brand-600 dark:text-brand-300">
                                正在导入游戏...
                            </p>
                            <p className="text-sm text-brand-400 dark:text-brand-500 mt-2">
                                这可能需要一些时间，请勿关闭窗口
                            </p>
                        </div>
                    )}

                    {/* Step: Result */}
                    {step === 'result' && importResult && (
                        <div className="space-y-6">
                            {/* Result Summary */}
                            <div className="flex gap-4">
                                <div className="flex-1 rounded-lg bg-green-50 dark:bg-green-900/20 p-4 text-center">
                                    <div className="i-mdi-check-circle text-3xl text-green-500 mx-auto mb-2" />
                                    <div className="text-2xl font-bold text-green-600 dark:text-green-400">
                                        {importResult.success}
                                    </div>
                                    <div className="text-sm text-green-700 dark:text-green-300">成功导入</div>
                                </div>
                                {importResult.skipped > 0 && (
                                    <div className="flex-1 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 p-4 text-center">
                                        <div className="i-mdi-skip-next-circle text-3xl text-yellow-500 mx-auto mb-2" />
                                        <div className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">
                                            {importResult.skipped}
                                        </div>
                                        <div className="text-sm text-yellow-700 dark:text-yellow-300">已跳过</div>
                                    </div>
                                )}
                                {importResult.failed > 0 && (
                                    <div className="flex-1 rounded-lg bg-red-50 dark:bg-red-900/20 p-4 text-center">
                                        <div className="i-mdi-close-circle text-3xl text-red-500 mx-auto mb-2" />
                                        <div className="text-2xl font-bold text-red-600 dark:text-red-400">
                                            {importResult.failed}
                                        </div>
                                        <div className="text-sm text-red-700 dark:text-red-300">导入失败</div>
                                    </div>
                                )}
                            </div>

                            {/* Skipped Names */}
                            {importResult.skipped_names && importResult.skipped_names.length > 0 && (
                                <div className="rounded-lg border border-yellow-200 dark:border-yellow-800 p-4">
                                    <h4 className="font-medium text-yellow-700 dark:text-yellow-400 mb-2">
                                        跳过的游戏:
                                    </h4>
                                    <div className="max-h-[150px] overflow-y-auto">
                                        <ul className="text-sm text-yellow-600 dark:text-yellow-300 space-y-1">
                                            {importResult.skipped_names.map((name, i) => (
                                                <li key={i}>• {name}</li>
                                            ))}
                                        </ul>
                                    </div>
                                </div>
                            )}

                            {/* Failed Names */}
                            {importResult.failed_names && importResult.failed_names.length > 0 && (
                                <div className="rounded-lg border border-red-200 dark:border-red-800 p-4">
                                    <h4 className="font-medium text-red-700 dark:text-red-400 mb-2">
                                        导入失败的游戏:
                                    </h4>
                                    <ul className="text-sm text-red-600 dark:text-red-300 space-y-1">
                                        {importResult.failed_names.map((name, i) => (
                                            <li key={i}>• {name}</li>
                                        ))}
                                    </ul>
                                </div>
                            )}

                            {/* Close Button */}
                            <div className="flex justify-center">
                                <button
                                    onClick={resetAndClose}
                                    className="rounded-lg bg-purple-600 px-8 py-2.5 text-sm font-medium text-white hover:bg-purple-700"
                                >
                                    完成
                                </button>
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}
