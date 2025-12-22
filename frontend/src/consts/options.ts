import { enums } from '../../wailsjs/go/models'

export const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '未开始', value: enums.GameStatus.NOT_STARTED },
  { label: '游玩中', value: enums.GameStatus.PLAYING },
  { label: '已通关', value: enums.GameStatus.COMPLETED },
  { label: '搁置', value: enums.GameStatus.ON_HOLD },
]

export const sortOptions = [
  { label: '名称', value: 'name' },
  { label: '添加时间', value: 'created_at' },
]
