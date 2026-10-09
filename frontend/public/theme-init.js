(function() {
            const storageKey = 'theme-preference'
            const getTheme = () => {
                if (localStorage.getItem(storageKey)) {
                    return localStorage.getItem(storageKey)
                }
                return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
            }
            // 立即应用主题（在页面渲染前添加 dark 类）
            if (getTheme() === 'dark') {
                document.documentElement.classList.add('dark')
            }
        })()
