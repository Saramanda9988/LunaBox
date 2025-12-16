import { createRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Route as rootRoute } from './__root'
import {
  GetCategories,
  AddCategory,
  DeleteCategory,
} from '../../wailsjs/go/service/CategoryService'
import { vo } from '../../wailsjs/go/models'
import { CategoryCard } from '../components/card/CategoryCard'
import { AddCategoryModal } from '../components/modal/AddCategoryModal'
import { FilterBar } from '../components/FilterBar'

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/categories',
  component: CategoriesPage,
})

function CategoriesPage() {
  const [categories, setCategories] = useState<vo.CategoryVO[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isAddCategoryModalOpen, setIsAddCategoryModalOpen] = useState(false)
  const [newCategoryName, setNewCategoryName] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const [sortBy, setSortBy] = useState<'name' | 'game_count' | 'created_at' | 'updated_at'>('name')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc')

  useEffect(() => {
    loadCategories()
  }, [])

  const loadCategories = async () => {
    try {
      const result = await GetCategories()
      setCategories(result || [])
    } catch (error) {
      console.error('Failed to load categories:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const handleAddCategory = async () => {
    if (!newCategoryName.trim()) return
    try {
      await AddCategory(newCategoryName)
      setNewCategoryName('')
      setIsAddCategoryModalOpen(false)
      await loadCategories()
    } catch (error) {
      console.error('Failed to add category:', error)
    }
  }

  const handleDeleteCategory = async (e: React.MouseEvent, category: vo.CategoryVO) => {
    e.stopPropagation()
    if (!confirm(`确定要删除收藏夹 "${category.name}" 吗？`)) return
    try {
      await DeleteCategory(category.id)
      await loadCategories()
    } catch (error) {
      console.error('Failed to delete category:', error)
    }
  }

  const filteredCategories = categories
    .filter((category) => {
      if (!searchQuery) return true
      return category.name.toLowerCase().includes(searchQuery.toLowerCase())
    })
    .sort((a, b) => {
      let comparison = 0
      switch (sortBy) {
        case 'name':
          comparison = a.name.localeCompare(b.name)
          break
        case 'game_count':
          comparison = (a.game_count || 0) - (b.game_count || 0)
          break
        case 'created_at':
          comparison = (a.created_at || '').toString().localeCompare((b.created_at || '').toString())
          break
        case 'updated_at':
          comparison = (a.updated_at || '').toString().localeCompare((b.updated_at || '').toString())
          break
      }
      return sortOrder === 'asc' ? comparison : -comparison
    })

  if (isLoading) {
    return <div className="flex h-full items-center justify-center">Loading...</div>
  }

  return (
    <div className="h-full w-full overflow-y-auto p-8">
      <div className="flex items-center justify-between">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white">收藏</h1>
      </div>
      
      <FilterBar
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        searchPlaceholder="搜索收藏夹..."
        sortBy={sortBy}
        onSortByChange={(val) => setSortBy(val as any)}
        sortOptions={[
          { label: '名称', value: 'name' },
          { label: '游戏数量', value: 'game_count' },
          { label: '创建时间', value: 'created_at' },
          { label: '更新时间', value: 'updated_at' },
        ]}
        sortOrder={sortOrder}
        onSortOrderChange={setSortOrder}
        actionButton={
          <button
            onClick={() => setIsAddCategoryModalOpen(true)}
            className="flex items-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-4 focus:ring-blue-300 dark:bg-blue-600 dark:hover:bg-blue-700 dark:focus:ring-blue-800"
          >
            <div className="i-mdi-plus mr-2 text-lg" />
            新建收藏夹
          </button>
        }
      />

      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {filteredCategories.map((category) => (
          <CategoryCard
            key={category.id}
            category={category}
            onDelete={(e) => handleDeleteCategory(e, category)}
          />
        ))}
      </div>

      <AddCategoryModal
        isOpen={isAddCategoryModalOpen}
        value={newCategoryName}
        onChange={setNewCategoryName}
        onClose={() => setIsAddCategoryModalOpen(false)}
        onSubmit={handleAddCategory}
      />
    </div>
  )
}
