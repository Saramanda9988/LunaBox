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

  if (isLoading) {
    return <div className="flex h-full items-center justify-center">Loading...</div>
  }

  return (
    <div className="h-full w-full overflow-y-auto p-8">
      <div className="flex items-center justify-between">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white">收藏</h1>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-4 my-4">
        <button
          onClick={() => setIsAddCategoryModalOpen(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 flex items-center gap-2"
        >
          <div className="i-mdi-plus" />
          新建收藏夹
        </button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {categories.map((category) => (
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
