import { useState, useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import toast from 'react-hot-toast'
import { vo, enums, models } from '../../../wailsjs/go/models'
import {
    SelectLibraryDirectory,
    ScanLibraryDirectory,
    FetchMetadataForCandidate,
    BatchImportGames
} from '../../../wailsjs/go/service/ImportService'
import { FetchMetadataByName, FetchMetadata } from '../../../wailsjs/go/service/GameService'
import { service } from '../../../wailsjs/go/models'

interface BatchImportModalProps {
    isOpen: boolean
    onClose: () => void
    onImportComplete: () => void
}

type Step = 'select' | 'scan' | 'preview' | 'match' | 'importing' | 'result'

interface LocalCandidate {
    folderPath: string
    folderName: string
    executables: string[]
    selectedExe: string
    searchName: string
    isSelected: boolean
    matchedGame: models.Game | null
    matchSource: enums.SourceType | null
    matchStatus: 'pending' | 'matched' | 'not_found' | 'error' | 'manual'
    allMatches?: vo.GameMetadataFromWebVO[]
}

export function BatchImportModal({ isOpen, onClose, onImportComplete }: BatchImportModalProps) {
    const [step, setStep] = useState<Step>('select')
    const [libraryPath, setLibraryPath] = useState('')
    const [candidates, setCandidates] = useState<LocalCandidate[]>([])
    const [importResult, setImportResult] = useState<service.ImportResult | null>(null)
    const [isLoading, setIsLoading] = useState(false)
    const [matchProgress, setMatchProgress] = useState({ current: 0, total: 0, gameName: '' })
    
    // 用于中断匹配过程的标志
    const abortMatchRef = useRef(false)
    
    // 手动选择弹窗状态
    const [showManualSelect, setShowManualSelect] = useState(false)
    const [manualSelectIndex, setManualSelectIndex] = useState<number | null>(null)
    const [manualMatches, setManualMatches] = useState<vo.GameMetadataFromWebVO[]>([])
    const [isSearching, setIsSearching] = useState(false)
    const [manualId, setManualId] = useState('')
    const [manualSource, setManualSource] = useState<enums.SourceType>(enums.SourceType.BANGUMI)

    if (!isOpen) return null

    const handleSelectDirectory = async () => {
        try {
            const path = await SelectLibraryDirectory()
            if (path) {
                setLibraryPath(path)
                setStep('scan')
                setIsLoading(true)
                
                try {
                    const scanned = await ScanLibraryDirectory(path)
                    const localCandidates: LocalCandidate[] = (scanned || []).map(c => ({
                        folderPath: c.folder_path,
                        folderName: c.folder_name,
                        executables: c.executables || [],
                        selectedExe: c.selected_exe,
                        searchName: c.search_name,
                        isSelected: true,
                        matchedGame: null,
                        matchSource: null,
                        matchStatus: 'pending'
                    }))
                    setCandidates(localCandidates)
                    setStep('preview')
                } catch (error) {
                    console.error('Failed to scan directory:', error)
                    toast.error('扫描目录失败')
                    setStep('select')
                } finally {
                    setIsLoading(false)
                }
            }
        } catch (error) {
            console.error('Failed to select directory:', error)
            toast.error('选择目录失败')
        }
    }

    const handleStartMatch = async () => {
        setStep('match')
        abortMatchRef.current = false
        
        // 只匹配选中且状态为 pending 的项目（跳过已手动匹配的）
        const toMatchCandidates = candidates.filter(c => c.isSelected && c.matchStatus === 'pending')
        setMatchProgress({ current: 0, total: toMatchCandidates.length, gameName: '' })

        const updatedCandidates = [...candidates]
        let matchedCount = 0
        
        for (let i = 0; i < candidates.length; i++) {
            // 检查是否需要中断
            if (abortMatchRef.current) {
                break
            }
            
            // 跳过未选中或已经匹配过的（包括手动匹配）
            if (!candidates[i].isSelected || candidates[i].matchStatus === 'matched' || candidates[i].matchStatus === 'manual') {
                continue
            }

            matchedCount++
            setMatchProgress(prev => ({
                ...prev,
                current: matchedCount,
                gameName: candidates[i].searchName
            }))

            try {
                // 使用现有的 FetchMetadataByName 获取所有源的结果
                const results = await FetchMetadataByName(candidates[i].searchName)
                
                if (results && results.length > 0) {
                    // 按优先级选择：Bangumi > VNDB > Ymgal
                    const priorityOrder = [enums.SourceType.BANGUMI, enums.SourceType.VNDB, enums.SourceType.YMGAL]
                    let bestMatch: vo.GameMetadataFromWebVO | null = null
                    
                    for (const source of priorityOrder) {
                        const match = results.find(r => r.Source === source && r.Game)
                        if (match) {
                            bestMatch = match
                            break
                        }
                    }

                    if (bestMatch && bestMatch.Game) {
                        updatedCandidates[i] = {
                            ...updatedCandidates[i],
                            matchedGame: bestMatch.Game,
                            matchSource: bestMatch.Source,
                            matchStatus: 'matched',
                            allMatches: results
                        }
                    } else {
                        updatedCandidates[i] = {
                            ...updatedCandidates[i],
                            matchStatus: 'not_found',
                            allMatches: results
                        }
                    }
                } else {
                    updatedCandidates[i] = {
                        ...updatedCandidates[i],
                        matchStatus: 'not_found'
                    }
                }
            } catch (error) {
                console.error(`Failed to match ${candidates[i].searchName}:`, error)
                updatedCandidates[i] = {
                    ...updatedCandidates[i],
                    matchStatus: 'error'
                }
            }

            setCandidates([...updatedCandidates])

            // 请求间隔，避免触发限流（如果已中断则不等待）
            if (!abortMatchRef.current) {
                await new Promise(resolve => setTimeout(resolve, 1500))
            }
        }

        // 只有在未中断的情况下才切换到预览步骤
        if (!abortMatchRef.current) {
            setStep('preview')
        }
    }

    const handleImport = async () => {
        setStep('importing')
        setIsLoading(true)

        try {
            // 转换为后端需要的格式
            const importCandidates: vo.BatchImportCandidate[] = candidates
                .filter(c => c.isSelected)
                .map(c => {
                    const candidate = new vo.BatchImportCandidate({
                        folder_path: c.folderPath,
                        folder_name: c.folderName,
                        executables: c.executables,
                        selected_exe: c.selectedExe,
                        search_name: c.searchName,
                        is_selected: c.isSelected,
                        match_status: c.matchStatus
                    })
                    if (c.matchedGame) {
                        candidate.matched_game = c.matchedGame
                    }
                    if (c.matchSource) {
                        candidate.match_source = c.matchSource
                    }
                    return candidate
                })

            const result = await BatchImportGames(importCandidates)
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

    const toggleCandidate = (index: number) => {
        const updated = [...candidates]
        updated[index].isSelected = !updated[index].isSelected
        setCandidates(updated)
    }

    const updateSearchName = (index: number, name: string) => {
        const updated = [...candidates]
        updated[index].searchName = name
        updated[index].matchStatus = 'pending'
        updated[index].matchedGame = null
        updated[index].matchSource = null
        setCandidates(updated)
    }

    const updateSelectedExe = (index: number, exe: string) => {
        const updated = [...candidates]
        updated[index].selectedExe = exe
        setCandidates(updated)
    }

    const openManualSelect = async (index: number) => {
        setManualSelectIndex(index)
        setManualMatches(candidates[index].allMatches || [])
        setShowManualSelect(true)
        setManualId('')
        
        // 如果没有缓存的匹配结果，重新搜索
        if (!candidates[index].allMatches || candidates[index].allMatches.length === 0) {
            setIsSearching(true)
            try {
                const results = await FetchMetadataByName(candidates[index].searchName)
                setManualMatches(results || [])
            } catch (error) {
                console.error('Failed to search:', error)
            } finally {
                setIsSearching(false)
            }
        }
    }

    const selectManualMatch = (game: models.Game, source: enums.SourceType) => {
        if (manualSelectIndex !== null) {
            const updated = [...candidates]
            updated[manualSelectIndex] = {
                ...updated[manualSelectIndex],
                matchedGame: game,
                matchSource: source,
                matchStatus: 'manual'
            }
            setCandidates(updated)
        }
        setShowManualSelect(false)
        setManualSelectIndex(null)
    }

    const handleSearchById = async () => {
        if (!manualId || manualSelectIndex === null) return
        setIsSearching(true)
        try {
            const request = new vo.MetadataRequest({
                source: manualSource,
                id: manualId
            })
            const game = await FetchMetadata(request)
            if (game && game.name) {
                selectManualMatch(game, manualSource)
            } else {
                toast.error('未找到游戏')
            }
        } catch (error) {
            console.error('Failed to fetch by ID:', error)
            toast.error('获取失败')
        } finally {
            setIsSearching(false)
        }
    }

    const resetAndClose = () => {
        // 中断正在进行的匹配
        abortMatchRef.current = true
        
        setStep('select')
        setLibraryPath('')
        setCandidates([])
        setImportResult(null)
        setMatchProgress({ current: 0, total: 0, gameName: '' })
        setShowManualSelect(false)
        setManualSelectIndex(null)
        onClose()
    }

    const selectedCount = candidates.filter(c => c.isSelected).length
    // 已匹配包括自动匹配和手动匹配
    const matchedCount = candidates.filter(c => c.isSelected && (c.matchStatus === 'matched' || c.matchStatus === 'manual')).length
    const notFoundCount = candidates.filter(c => c.isSelected && c.matchStatus === 'not_found').length
    const pendingCount = candidates.filter(c => c.isSelected && c.matchStatus === 'pending').length

    return createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
            <div className="w-full max-w-4xl max-h-[90vh] rounded-xl bg-white shadow-2xl dark:bg-brand-800 flex flex-col">
                {/* Header */}
                <div className="flex items-center justify-between p-6 border-b border-brand-200 dark:border-brand-700">
                    <div className="flex items-center gap-3">
                        <div className="i-mdi-folder-multiple text-3xl text-success-500" />
                        <h2 className="text-2xl font-bold text-brand-900 dark:text-white">
                            批量导入游戏库
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
                    {/* Step: Select Directory */}
                    {step === 'select' && (
                        <div className="space-y-6">
                            <div className="text-center py-8">
                                <div className="i-mdi-folder-open text-6xl text-brand-400 mx-auto mb-4" />
                                <p className="text-brand-600 dark:text-brand-300 mb-2">
                                    选择您的游戏库目录
                                </p>
                                <p className="text-sm text-brand-400 dark:text-brand-500">
                                    程序将扫描一级子文件夹，每个包含可执行文件的文件夹将被识别为一个游戏
                                </p>
                            </div>

                            <button
                                onClick={handleSelectDirectory}
                                disabled={isLoading}
                                className="flex w-full items-center justify-center rounded-lg py-4 text-white transition disabled:opacity-50 bg-success-500 hover:bg-success-600"
                            >
                                <div className="i-mdi-folder-search mr-2 text-xl" />
                                选择游戏库目录
                            </button>
                        </div>
                    )}

                    {/* Step: Scanning */}
                    {step === 'scan' && (
                        <div className="py-12 text-center">
                            <div className="i-mdi-loading animate-spin text-5xl mx-auto mb-4 text-success-500" />
                            <p className="text-lg text-brand-600 dark:text-brand-300">
                                正在扫描目录...
                            </p>
                            <p className="text-sm text-brand-400 dark:text-brand-500 mt-2">
                                {libraryPath}
                            </p>
                        </div>
                    )}

                    {/* Step: Preview & Match */}
                    {step === 'preview' && (
                        <div className="space-y-4">
                            {/* Summary */}
                            <div className="flex gap-4">
                                <div className="flex-1 rounded-lg bg-neutral-50 dark:bg-neutral-900/20 p-4 text-center">
                                    <div className="text-3xl font-bold text-neutral-600 dark:text-neutral-400">
                                        {candidates.length}
                                    </div>
                                    <div className="text-sm text-neutral-700 dark:text-neutral-300">
                                        检测到
                                    </div>
                                </div>
                                <div className="flex-1 rounded-lg bg-success-50 dark:bg-success-900/20 p-4 text-center">
                                    <div className="text-3xl font-bold text-success-600 dark:text-success-400">
                                        {matchedCount}
                                    </div>
                                    <div className="text-sm text-success-700 dark:text-success-300">
                                        已匹配
                                    </div>
                                </div>
                                {notFoundCount > 0 && (
                                    <div className="flex-1 rounded-lg bg-orange-50 dark:bg-orange-900/20 p-4 text-center">
                                        <div className="text-3xl font-bold text-orange-600 dark:text-orange-400">
                                            {notFoundCount}
                                        </div>
                                        <div className="text-sm text-orange-700 dark:text-orange-300">
                                            未匹配
                                        </div>
                                    </div>
                                )}
                                {pendingCount > 0 && (
                                    <div className="flex-1 rounded-lg bg-gray-50 dark:bg-gray-900/20 p-4 text-center">
                                        <div className="text-3xl font-bold text-gray-600 dark:text-gray-400">
                                            {pendingCount}
                                        </div>
                                        <div className="text-sm text-gray-700 dark:text-gray-300">
                                            待匹配
                                        </div>
                                    </div>
                                )}
                            </div>

                            {/* Candidate List */}
                            <div className="max-h-[400px] overflow-y-auto rounded-lg border border-brand-200 dark:border-brand-700">
                                {candidates.length === 0 ? (
                                    <div className="p-8 text-center text-brand-400">
                                        未检测到游戏文件夹
                                    </div>
                                ) : (
                                    <table className="w-full">
                                        <thead className="top-0 bg-brand-50 dark:bg-brand-700">
                                            <tr>
                                                <th className="px-3 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300 w-10">
                                                    <input
                                                        type="checkbox"
                                                        checked={candidates.every(c => c.isSelected)}
                                                        onChange={(e) => {
                                                            const updated = candidates.map(c => ({
                                                                ...c,
                                                                isSelected: e.target.checked
                                                            }))
                                                            setCandidates(updated)
                                                        }}
                                                    />
                                                </th>
                                                <th className="px-3 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300">
                                                    搜索名称
                                                </th>
                                                <th className="px-3 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300">
                                                    启动程序
                                                </th>
                                                <th className="px-3 py-2 text-center text-sm font-medium text-brand-600 dark:text-brand-300 w-32">
                                                    匹配状态
                                                </th>
                                                <th className="px-3 py-2 text-center text-sm font-medium text-brand-600 dark:text-brand-300 w-20">
                                                    操作
                                                </th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-brand-100 dark:divide-brand-700">
                                            {candidates.map((candidate, index) => (
                                                <tr
                                                    key={index}
                                                    className={`${!candidate.isSelected ? 'opacity-50' : 'hover:bg-brand-50 dark:hover:bg-brand-750'}`}
                                                >
                                                    <td className="px-3 py-2">
                                                        <input
                                                            type="checkbox"
                                                            checked={candidate.isSelected}
                                                            onChange={() => toggleCandidate(index)}
                                                        />
                                                    </td>
                                                    <td className="px-3 py-2">
                                                        <input
                                                            type="text"
                                                            value={candidate.searchName}
                                                            onChange={(e) => updateSearchName(index, e.target.value)}
                                                            className="w-full bg-transparent border-b border-transparent hover:border-brand-300 focus:border-neutral-500 focus:outline-none text-sm text-brand-900 dark:text-white"
                                                        />
                                                        {candidate.matchedGame && (
                                                            <div className="text-xs text-success-600 dark:text-success-400 mt-1 flex items-center gap-1">
                                                                <span>→ {candidate.matchedGame.name}</span>
                                                                <span className="text-brand-400">({candidate.matchSource})</span>
                                                            </div>
                                                        )}
                                                    </td>
                                                    <td className="px-3 py-2">
                                                        {candidate.executables.length > 1 ? (
                                                            <select
                                                                value={candidate.selectedExe}
                                                                onChange={(e) => updateSelectedExe(index, e.target.value)}
                                                                className="w-full bg-transparent text-sm text-brand-700 dark:text-brand-300 border border-brand-200 dark:border-brand-600 rounded px-2 py-1"
                                                            >
                                                                {candidate.executables.map((exe, i) => (
                                                                    <option key={i} value={exe}>
                                                                        {exe.split(/[/\\]/).pop()}
                                                                    </option>
                                                                ))}
                                                            </select>
                                                        ) : (
                                                            <span className="text-sm text-brand-500 dark:text-brand-400">
                                                                {candidate.selectedExe.split(/[/\\]/).pop()}
                                                            </span>
                                                        )}
                                                    </td>
                                                    <td className="px-3 py-2 text-center">
                                                        {candidate.matchStatus === 'pending' && (
                                                            <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-gray-900/30 dark:text-gray-400">
                                                                <div className="i-mdi-clock-outline mr-1" />
                                                                待匹配
                                                            </span>
                                                        )}
                                                        {(candidate.matchStatus === 'matched' || candidate.matchStatus === 'manual') && (
                                                            <span className="inline-flex items-center rounded-full bg-success-100 px-2 py-1 text-xs text-success-700 dark:bg-success-900/30 dark:text-success-400">
                                                                <div className="i-mdi-check-circle mr-1" />
                                                                已匹配
                                                            </span>
                                                        )}
                                                        {candidate.matchStatus === 'not_found' && (
                                                            <span className="inline-flex items-center rounded-full bg-orange-100 px-2 py-1 text-xs text-orange-700 dark:bg-orange-900/30 dark:text-orange-400">
                                                                <div className="i-mdi-alert-circle mr-1" />
                                                                未找到
                                                            </span>
                                                        )}
                                                        {candidate.matchStatus === 'error' && (
                                                            <span className="inline-flex items-center rounded-full bg-error-100 px-2 py-1 text-xs text-error-700 dark:bg-error-900/30 dark:text-error-400">
                                                                <div className="i-mdi-close-circle mr-1" />
                                                                错误
                                                            </span>
                                                        )}
                                                    </td>
                                                    <td className="px-3 py-2 text-center">
                                                        <button
                                                            onClick={() => openManualSelect(index)}
                                                            className="text-neutral-500 hover:text-neutral-700 text-sm"
                                                            title="手动选择"
                                                        >
                                                            <div className="i-mdi-pencil text-lg" />
                                                        </button>
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
                                <div className="flex gap-3">
                                    {pendingCount > 0 && (
                                        <button
                                            onClick={handleStartMatch}
                                            className="rounded-lg px-5 py-2.5 text-sm font-medium text-white bg-neutral-600 hover:bg-neutral-700"
                                        >
                                            开始匹配元数据
                                        </button>
                                    )}
                                    <button
                                        onClick={handleImport}
                                        disabled={selectedCount === 0}
                                        className="rounded-lg px-5 py-2.5 text-sm font-medium text-white disabled:opacity-50 bg-success-600 hover:bg-success-700"
                                    >
                                        导入 {selectedCount} 个游戏
                                    </button>
                                </div>
                            </div>
                        </div>
                    )}

                    {/* Step: Matching */}
                    {step === 'match' && (
                        <div className="py-12 text-center">
                            <div className="i-mdi-loading animate-spin text-5xl mx-auto mb-4 text-neutral-500" />
                            <p className="text-lg text-brand-600 dark:text-brand-300">
                                正在匹配元数据...
                            </p>
                            <p className="text-sm text-brand-400 dark:text-brand-500 mt-2">
                                {matchProgress.current} / {matchProgress.total}
                            </p>
                            <p className="text-sm text-neutral-500 mt-2">
                                {matchProgress.gameName}
                            </p>
                            <div className="w-full max-w-md mx-auto mt-4 bg-brand-200 dark:bg-brand-700 rounded-full h-2">
                                <div
                                    className="bg-neutral-500 h-2 rounded-full transition-all duration-300"
                                    style={{ width: `${matchProgress.total > 0 ? (matchProgress.current / matchProgress.total) * 100 : 0}%` }}
                                />
                            </div>
                            <p className="text-xs text-brand-400 mt-4">
                                为避免触发限流，匹配速度可能较慢，请耐心等待
                            </p>
                        </div>
                    )}

                    {/* Step: Importing */}
                    {step === 'importing' && (
                        <div className="py-12 text-center">
                            <div className="i-mdi-loading animate-spin text-5xl mx-auto mb-4 text-success-500" />
                            <p className="text-lg text-brand-600 dark:text-brand-300">
                                正在导入游戏...
                            </p>
                        </div>
                    )}

                    {/* Step: Result */}
                    {step === 'result' && importResult && (
                        <div className="space-y-6">
                            <div className="flex gap-4">
                                <div className="flex-1 rounded-lg bg-success-50 dark:bg-success-900/20 p-4 text-center">
                                    <div className="i-mdi-check-circle text-3xl text-success-500 mx-auto mb-2" />
                                    <div className="text-2xl font-bold text-success-600 dark:text-success-400">
                                        {importResult.success}
                                    </div>
                                    <div className="text-sm text-success-700 dark:text-success-300">成功导入</div>
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
                                    <div className="flex-1 rounded-lg bg-error-50 dark:bg-error-900/20 p-4 text-center">
                                        <div className="i-mdi-close-circle text-3xl text-error-500 mx-auto mb-2" />
                                        <div className="text-2xl font-bold text-error-600 dark:text-error-400">
                                            {importResult.failed}
                                        </div>
                                        <div className="text-sm text-error-700 dark:text-error-300">导入失败</div>
                                    </div>
                                )}
                            </div>

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

                            {importResult.failed_names && importResult.failed_names.length > 0 && (
                                <div className="rounded-lg border border-error-200 dark:border-error-800 p-4">
                                    <h4 className="font-medium text-error-700 dark:text-error-400 mb-2">
                                        导入失败的游戏:
                                    </h4>
                                    <ul className="text-sm text-error-600 dark:text-error-300 space-y-1">
                                        {importResult.failed_names.map((name, i) => (
                                            <li key={i}>• {name}</li>
                                        ))}
                                    </ul>
                                </div>
                            )}

                            <div className="flex justify-center">
                                <button
                                    onClick={resetAndClose}
                                    className="rounded-lg px-8 py-2.5 text-sm font-medium text-white bg-success-600 hover:bg-success-700"
                                >
                                    完成
                                </button>
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {/* Manual Select Modal */}
            {showManualSelect && manualSelectIndex !== null && (
                <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/50">
                    <div className="w-full max-w-2xl max-h-[80vh] rounded-xl bg-white shadow-2xl dark:bg-brand-800 flex flex-col">
                        <div className="flex items-center justify-between p-4 border-b border-brand-200 dark:border-brand-700">
                            <h3 className="text-lg font-bold text-brand-900 dark:text-white">
                                手动选择: {candidates[manualSelectIndex].searchName}
                            </h3>
                            <button
                                onClick={() => setShowManualSelect(false)}
                                className="i-mdi-close text-xl text-brand-500 hover:text-brand-700"
                            />
                        </div>
                        
                        <div className="flex-1 overflow-y-auto p-4 space-y-4">
                            {isSearching ? (
                                <div className="py-8 text-center">
                                    <div className="i-mdi-loading animate-spin text-3xl mx-auto mb-2 text-neutral-500" />
                                    <p className="text-brand-400">搜索中...</p>
                                </div>
                            ) : (
                                <>
                                    {/* 匹配结果 */}
                                    <div className="flex flex-wrap gap-3">
                                        {manualMatches.filter(m => m.Game).map((match, idx) => (
                                            <div
                                                key={idx}
                                                onClick={() => selectManualMatch(match.Game!, match.Source)}
                                                className="w-36 cursor-pointer rounded-lg border border-brand-200 p-2 transition hover:border-neutral-500 hover:shadow-md dark:border-brand-700"
                                            >
                                                <div className="aspect-[3/4] w-full overflow-hidden rounded-md bg-brand-200 dark:bg-brand-700">
                                                    {match.Game!.cover_url ? (
                                                        <img src={match.Game!.cover_url} alt={match.Game!.name} className="h-full w-full object-cover" referrerPolicy="no-referrer" />
                                                    ) : (
                                                        <div className="flex h-full items-center justify-center text-brand-400">
                                                            <div className="i-mdi-image-off text-3xl" />
                                                        </div>
                                                    )}
                                                </div>
                                                <h4 className="mt-1 truncate text-xs font-bold text-brand-900 dark:text-white" title={match.Game!.name}>
                                                    {match.Game!.name}
                                                </h4>
                                                <p className="text-xs text-brand-400">{match.Source}</p>
                                            </div>
                                        ))}
                                    </div>

                                    {manualMatches.length === 0 && (
                                        <p className="text-center text-brand-400 py-4">未找到匹配结果</p>
                                    )}

                                    {/* 手动输入ID */}
                                    <div className="border-t border-brand-200 dark:border-brand-700 pt-4 mt-4">
                                        <p className="text-sm text-brand-500 mb-3">通过 ID 查找:</p>
                                        <div className="flex gap-2">
                                            <select
                                                value={manualSource}
                                                onChange={(e) => setManualSource(e.target.value as enums.SourceType)}
                                                className="rounded border border-brand-300 bg-brand-50 px-2 py-1.5 text-sm dark:border-brand-600 dark:bg-brand-700"
                                            >
                                                <option value={enums.SourceType.BANGUMI}>Bangumi</option>
                                                <option value={enums.SourceType.VNDB}>VNDB</option>
                                                <option value={enums.SourceType.YMGAL}>月幕gal</option>
                                            </select>
                                            <input
                                                type="text"
                                                value={manualId}
                                                onChange={(e) => setManualId(e.target.value)}
                                                placeholder="输入 ID"
                                                className="flex-1 rounded border border-brand-300 bg-brand-50 px-3 py-1.5 text-sm dark:border-brand-600 dark:bg-brand-700"
                                            />
                                            <button
                                                onClick={handleSearchById}
                                                disabled={!manualId || isSearching}
                                                className="rounded bg-neutral-500 px-4 py-1.5 text-sm text-white hover:bg-neutral-600 disabled:opacity-50"
                                            >
                                                查找
                                            </button>
                                        </div>
                                    </div>

                                    {/* 跳过元数据 */}
                                    <button
                                        onClick={() => {
                                            const updated = [...candidates]
                                            updated[manualSelectIndex] = {
                                                ...updated[manualSelectIndex],
                                                matchedGame: null,
                                                matchSource: null,
                                                matchStatus: 'not_found'
                                            }
                                            setCandidates(updated)
                                            setShowManualSelect(false)
                                        }}
                                        className="w-full text-center text-sm text-brand-400 hover:text-brand-600 py-2"
                                    >
                                        不匹配元数据，仅导入路径
                                    </button>
                                </>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>,
        document.body
    )
}
