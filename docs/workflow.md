# 变更流程与自检清单

## Agent 变更流程（推荐顺序）

当被要求"实现一个功能/修复一个问题"时，按此顺序执行：

1. **定位落点**：优先在现有 route/service/utils 中找最贴近的位置（参考 [anchors.md](anchors.md)）
2. **复用模式**：沿用同目录已有写法（migration 幂等、service 注入、UI glass/dark）
3. **实现最小改动**：避免新增大层级抽象
4. **本地验证**：尽量运行已有 build task
5. **自检**：对照下方清单

---

## 自检清单（交付前）

### 前端

- [ ] 新增/修改的组件在 `light` 与 `dark` 下都可读
- [ ] 背景开启时 `data-glass="true"` 下观感不崩
- [ ] 没有把局部样式硬塞进 `style.css`（除非是全局不可避免项）
- [ ] 新增组件遵守 HeadlessUI/Radix 封装约束，没有直接引入大型 UI 框架
- [ ] 页面最外层盒子没有设置颜色与不透明度

### 后端

- [ ] 涉及 schema 变更时同时更新 `InitSchema` + 新 migration
- [ ] Migration 幂等且事务安全，空库也能跑通
- [ ] 没有引入非 Windows 平台的系统调用
- [ ] 底层操作优先 Wails/Go 标准库

### 通用

- [ ] 新增工具函数前已搜索并复用现有 `frontend/src/utils` 或 `internal/utils`
- [ ] 没有顺手重构或格式化不相关代码
