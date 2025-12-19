import { useState } from 'react'
import { service } from '../../../wailsjs/go/models'
import { SelectZipFile, ImportFromPotatoVN, PreviewImport } from '../../../wailsjs/go/service/ImportService'
import { toast } from 'react-hot-toast'

interface ImportModalProps {
    isOpen: boolean
    onClose: () => void
    onImportComplete: () => void
}

type Step = 'select' | 'preview' | 'importing' | 'result'

export function ImportModal({ isOpen, onClose, onImportComplete }: ImportModalProps) {
    const [step, setStep] = useState<Step>('select')
    const [zipPath, setZipPath] = useState('')
    const [previewGames, setPreviewGames] = useState<service.PreviewGame[]>([])
    const [importResult, setImportResult] = useState<service.ImportResult | null>(null)
    const [isLoading, setIsLoading] = useState(false)

    if (!isOpen) return null

    const handleSelectFile = async () => {
        try {
            const path = await SelectZipFile()
            if (path) {
                setZipPath(path)
                setIsLoading(true)
                try {
                    const games = await PreviewImport(path)
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
        if (!zipPath) return

        setStep('importing')
        setIsLoading(true)

        try {
            const result = await ImportFromPotatoVN(zipPath)
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
        setZipPath('')
        setPreviewGames([])
        setImportResult(null)
        onClose()
    }

    const newGamesCount = previewGames.filter(g => !g.exists).length
    const existingGamesCount = previewGames.filter(g => g.exists).length

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
            <div className="w-full max-w-2xl rounded-xl bg-white p-6 shadow-2xl dark:bg-brand-800">
                {/* Header */}
                <div className="flex items-center justify-between mb-6">
                    <div className="flex items-center gap-3">
                        <div className="i-mdi-database-import text-3xl text-blue-500" />
                        <h2 className="text-2xl font-bold text-brand-900 dark:text-white">
                            从 PotatoVN 导入
                        </h2>
                    </div>
                    <button
                        onClick={resetAndClose}
                        className="i-mdi-close text-2xl text-brand-500 p-1 rounded-lg
              hover:bg-brand-100 hover:text-brand-700 focus:outline-none
              dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
                    />
                </div>

                {/* Step: Select File */}
                {step === 'select' && (
                    <div className="space-y-6">
                        <div className="text-center py-8">
                            <div className="i-mdi-folder-zip text-6xl text-brand-400 mx-auto mb-4" />
                            <p className="text-brand-600 dark:text-brand-300 mb-2">
                                选择 PotatoVN 导出的 ZIP 文件
                            </p>
                            <p className="text-sm text-brand-400 dark:text-brand-500">
                                支持包含 data.galgames.json 的 PotatoVN 备份文件
                            </p>
                        </div>

                        <button
                            onClick={handleSelectFile}
                            disabled={isLoading}
                            className="flex w-full items-center justify-center rounded-lg bg-blue-500 py-4 text-white transition hover:bg-blue-600 disabled:opacity-50"
                        >
                            {isLoading ? (
                                <>
                                    <div className="i-mdi-loading animate-spin mr-2 text-xl" />
                                    加载中...
                                </>
                            ) : (
                                <>
                                    <div className="i-mdi-file-find mr-2 text-xl" />
                                    选择 ZIP 文件
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
                                    新游戏
                                </div>
                            </div>
                            <div className="flex-1 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 p-4 text-center">
                                <div className="text-3xl font-bold text-yellow-600 dark:text-yellow-400">
                                    {existingGamesCount}
                                </div>
                                <div className="text-sm text-yellow-700 dark:text-yellow-300">
                                    将跳过（已存在）
                                </div>
                            </div>
                        </div>

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
                                        {previewGames.map((game, index) => (
                                            <tr
                                                key={index}
                                                className={`${game.exists
                                                        ? 'opacity-50'
                                                        : 'hover:bg-brand-50 dark:hover:bg-brand-750'
                                                    }`}
                                            >
                                                <td className="px-4 py-3 text-sm text-brand-900 dark:text-white">
                                                    {game.name}
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
                                                    ) : (
                                                        <span className="inline-flex items-center rounded-full bg-green-100 px-2 py-1 text-xs text-green-700 dark:bg-green-900/30 dark:text-green-400">
                                                            <div className="i-mdi-plus-circle mr-1" />
                                                            新增
                                                        </span>
                                                    )}
                                                </td>
                                            </tr>
                                        ))}
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
                                className="rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                            >
                                导入 {newGamesCount} 个游戏
                            </button>
                        </div>
                    </div>
                )}

                {/* Step: Importing */}
                {step === 'importing' && (
                    <div className="py-12 text-center">
                        <div className="i-mdi-loading animate-spin text-5xl text-blue-500 mx-auto mb-4" />
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
                                    <div className="text-sm text-red-700 dark:text-red-300">失败</div>
                                </div>
                            )}
                        </div>

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
                                className="rounded-lg bg-blue-600 px-8 py-2.5 text-sm font-medium text-white hover:bg-blue-700"
                            >
                                完成
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    )
}
