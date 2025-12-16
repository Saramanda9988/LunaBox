import { defineConfig, presetIcons, presetWind3} from 'unocss'

export default defineConfig({
    presets: [
        presetWind3({
            // 推荐方式：使用 class 模式（与你现有的 App.tsx 逻辑兼容）
            // 也可以改为 'media' 让系统自动根据用户偏好切换
            dark: 'class'
        }),
        presetIcons(),
    ],
    theme: {
        colors: {
            brand: {
                100: '#f3f4f6',
                200: '#e5e7eb',
                300: '#d1d5db',
                400: '#9ca3af',
                500: '#6b7280',
                600: '#4b5563',
                700: '#44484eff',
                800: '#1c1e1fff',
                900: '#121416ff', // dark 模式下的背景颜色
            }
        }
    }
})