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

### 界面

- [ ] 新增/修改的界面在浅色与深色主题下都可读，开启自定义背景（玻璃模式）时观感不崩
- [ ] 颜色、字号用 `ui/theme` 的色板与 `components.Text`，没有凭截图估的新取值
- [ ] 文字用 `ui/locales` 的函数，新文字四种语言都写了并已 `pnpm i18n` 重新生成
- [ ] 时间相关的状态用帧时钟（`c.Now()`、`ev.Time`），测试用 `tt.Advance` 推进
- [ ] 组件不读页面状态，通过参数与回调交互；组件自己的临时状态用 `ui.Local`

### 后端

- [ ] 涉及 schema 变更时同时更新 `InitSchema` + 新 migration
- [ ] Migration 幂等且事务安全，空库也能跑通
- [ ] 没有引入非 Windows 平台的系统调用
- [ ] 界面框架能力通过 `appruntime.Runtime`，其余底层操作优先 Go 标准库
- [ ] 若新增/修改 `AppConfig` 字段，已同步检查 `LoadConfig/SaveConfig` 与 `ConfigService.UpdateAppConfig(...)`
- [ ] 若修改退出逻辑，已核对窗口关闭 / 界面退出流 / `OnShutdown` 的职责边界，没有把交互流程塞进 `OnShutdown`

### 通用

- [ ] 新增工具函数前已搜索并复用现有 `apps/desktop/ui`（如 `ui/timefmt`、`ui/components`）或 `packages/backend/utils`
- [ ] 没有顺手重构或格式化不相关代码
