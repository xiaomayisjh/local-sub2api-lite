/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主色调 - Anthropic 黏土橙 (Clay / Terracotta)
        primary: {
          50: '#fbf3ef',
          100: '#f6e5dc',
          200: '#ecc8b6',
          300: '#e0a88d',
          400: '#d17f5c',
          500: '#c15f3c',
          600: '#a94f30',
          700: '#8a3f27',
          800: '#6e3220',
          900: '#5a2a1c',
          950: '#321610'
        },
        // 辅助色 - 暖棕褐 (与 primary 形成同色温渐变)
        accent: {
          50: '#f8f6f1',
          100: '#efece2',
          200: '#ddd6c6',
          300: '#c5bca6',
          400: '#a89d83',
          500: '#8a7e64',
          600: '#6f6450',
          700: '#574e3f',
          800: '#3f3a30',
          900: '#2a2722',
          950: '#1a1814'
        },
        // 中性灰 - 暖石色 (覆盖 Tailwind 默认冷灰，全站统一暖色调)
        gray: {
          50: '#faf9f5',
          100: '#f0eee6',
          200: '#e3e1d8',
          300: '#d1cec2',
          400: '#b0ab9a',
          500: '#8c8676',
          600: '#6b6557',
          700: '#524d42',
          800: '#3a362e',
          900: '#262420',
          950: '#1a1814'
        },
        // 深色模式背景 - 暖深褐/炭
        dark: {
          50: '#f7f6f2',
          100: '#edebe4',
          200: '#d9d6cc',
          300: '#b8b3a6',
          400: '#8f8a7c',
          500: '#6b6557',
          600: '#4e4a40',
          700: '#3a362e',
          800: '#2a2722',
          900: '#1e1b17',
          950: '#161310'
        }
      },
      fontFamily: {
        sans: [
          'Styrene',
          'ui-sans-serif',
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        // 衬线字体 - 用于标题，呼应 Anthropic 官网 Tiempos/Copernicus 风格
        serif: [
          'Tiempos',
          'Copernicus',
          'Georgia',
          'Songti SC',
          'STSong',
          'Noto Serif SC',
          'ui-serif',
          'serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        glass: '0 8px 32px rgba(74, 60, 46, 0.10)',
        'glass-sm': '0 4px 16px rgba(74, 60, 46, 0.08)',
        glow: '0 0 18px rgba(193, 95, 60, 0.18)',
        'glow-lg': '0 0 32px rgba(193, 95, 60, 0.24)',
        card: '0 1px 3px rgba(60, 50, 40, 0.05), 0 1px 2px rgba(60, 50, 40, 0.07)',
        'card-hover': '0 10px 32px rgba(60, 50, 40, 0.10)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.12)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, #c15f3c 0%, #a94f30 100%)',
        'gradient-dark': 'linear-gradient(135deg, #2a2722 0%, #1e1b17 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.12) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient':
          'radial-gradient(at 40% 20%, rgba(193, 95, 60, 0.08) 0px, transparent 50%), radial-gradient(at 80% 0%, rgba(168, 157, 131, 0.06) 0px, transparent 50%), radial-gradient(at 0% 50%, rgba(193, 95, 60, 0.05) 0px, transparent 50%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 18px rgba(193, 95, 60, 0.18)' },
          '100%': { boxShadow: '0 0 28px rgba(193, 95, 60, 0.30)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
