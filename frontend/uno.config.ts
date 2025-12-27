import { defineConfig, presetIcons, presetWind3 } from 'unocss'

export default defineConfig({
    presets: [
        presetWind3({
            dark: 'class'
        }),
        presetIcons(),
    ],
    theme: {
        colors: {
            // 基础灰度色板
            brand: {
                50: '#fdfdfdff',
                100: '#f3f4f6',
                200: '#e5e7eb',
                300: '#d1d5db',
                400: '#9ca3af',
                500: '#6b7280',
                600: '#4b5563',
                700: '#44484eff',
                750: '#303235ff',
                800: '#1c1e1fff',
                900: '#121416ff',
            },
            // 主色调 (primary) - 月光紫
            primary: {
                50: '#F5F3FF',
                100: '#EDE9FE',
                200: '#DDD6FE',
                300: '#C4B5FD',
                400: '#A78BFA',
                500: '#7C6AEF',
                600: '#6D5DD3',
                700: '#5B4CB8',
                800: '#4A3D96',
                900: '#2E2660',
            },
            // 强调色 (Accent) - 星光蓝
            accent: {
                50: '#EFF6FF',
                100: '#DBEAFE',
                200: '#BFDBFE',
                300: '#93C5FD',
                400: '#60A5FA',
                500: '#3B82F6',
                600: '#2563EB',
                700: '#1D4ED8',
                800: '#1E40AF',
                900: '#1E3A8A',
            },
            // 中性色 (Neutral) - 月夜灰
            neutral: {
                50: '#F8FAFC',
                100: '#F1F5F9',
                200: '#E2E8F0',
                300: '#CBD5E1',
                400: '#94A3B8',
                500: '#64748B',
                600: '#475569',
                700: '#334155',
                800: '#1E293B',
                900: '#0F172A',
            },
            // 成功色 (Success) - 极光绿
            success: {
                50: '#ECFDF5',
                100: '#D1FAE5',
                200: '#A7F3D0',
                300: '#6EE7B7',
                400: '#34D399',
                500: '#10B981',
                600: '#059669',
                700: '#047857',
                800: '#065F46',
                900: '#064E3B',
            },
            // 警告色 (Warning) - 晨曦金
            warning: {
                50: '#FFFBEB',
                100: '#FEF3C7',
                200: '#FDE68A',
                300: '#FCD34D',
                400: '#FBBF24',
                500: '#F59E0B',
                600: '#D97706',
                700: '#B45309',
                800: '#92400E',
                900: '#78350F',
            },
            // 错误色 (Error) - 玫瑰红
            error: {
                50: '#FEF2F2',
                100: '#FEE2E2',
                200: '#FECACA',
                300: '#FCA5A5',
                400: '#F87171',
                500: '#EF4444',
                600: '#DC2626',
                700: '#B91C1C',
                800: '#991B1B',
                900: '#7F1D1D',
            },
            // 信息色 (Info) - 冰蓝
            info: {
                50: '#F0F9FF',
                100: '#E0F2FE',
                200: '#BAE6FD',
                300: '#7DD3FC',
                400: '#38BDF8',
                500: '#0EA5E9',
                600: '#0284C7',
                700: '#0369A1',
                800: '#075985',
                900: '#0C181D',
            },
        }
    }
})