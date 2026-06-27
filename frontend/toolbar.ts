(function () {
    const container = document.getElementById('proxy-toolbar-container');
    if (!container) return;

    const shadow = container.attachShadow({ mode: 'open' });

    const style = document.createElement('style');
    style.textContent = `
        :host {
            all: initial;
            display: block;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 48px;
            z-index: 2147483647;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            box-sizing: border-box;
        }
        .toolbar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            height: 100%;
            padding: 0 12px;
            background: rgba(18, 18, 20, 0.85);
            backdrop-filter: blur(12px);
            -webkit-backdrop-filter: blur(12px);
            border-bottom: 1px solid rgba(255, 255, 255, 0.1);
            color: #fafafa;
            user-select: none;
            box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
            box-sizing: border-box;
            transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1);
            transform: translateY(0);
        }
        .toolbar.hidden {
            transform: translateY(-48px);
        }
        .left-section {
            display: flex;
            align-items: center;
            gap: 8px;
            flex: 1;
            min-width: 0;
            margin-right: 12px;
        }
        .logo {
            font-size: 16px;
            text-decoration: none;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            width: 32px;
            height: 32px;
            border-radius: 50%;
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid rgba(255, 255, 255, 0.1);
            color: #fafafa;
            transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
            box-sizing: border-box;
            flex-shrink: 0;
        }
        .logo:hover {
            background: rgba(255, 255, 255, 0.15);
            transform: translateY(-1px);
        }
        .logo:active {
            transform: translateY(0);
        }
        .url-info {
            color: #a1a1aa;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            cursor: pointer;
            transition: color 0.2s;
            flex: 1;
            min-width: 0;
            font-size: 12px;
        }
        .url-info:hover {
            color: #ffffff;
        }
        .right-section {
            display: flex;
            align-items: center;
            gap: 6px;
            flex-shrink: 0;
        }
        .btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            height: 32px;
            width: 32px;
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 50%;
            background: rgba(255, 255, 255, 0.05);
            color: #fafafa;
            cursor: pointer;
            font-size: 15px;
            transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
            text-decoration: none;
            box-sizing: border-box;
            flex-shrink: 0;
        }
        .btn:hover {
            background: rgba(255, 255, 255, 0.15);
            color: #ffffff;
            border-color: rgba(255, 255, 255, 0.2);
            transform: translateY(-1px);
        }
        .btn:active {
            transform: translateY(0);
        }
        .btn.active {
            background: #2563eb;
            color: #ffffff;
            border-color: #3b82f6;
        }
        .btn.active:hover {
            background: #1d4ed8;
        }
    `;
    shadow.appendChild(style);

    const toolbar = document.createElement('div');
    toolbar.className = 'toolbar';

    const targetURL: string = (window as Window & { __PROXY_TARGET_URL__?: string }).__PROXY_TARGET_URL__ ?? '';

    toolbar.innerHTML = `
        <div class="left-section">
            <a href="/" target="_self" class="logo" title="ホームへ戻る">🏠</a>
            <div class="url-info" id="url-text" title="${targetURL}">
                ${targetURL}
            </div>
        </div>
        <div class="right-section">
            <button class="btn" id="btn-style" title="リーダーモード (CSS無効化)">📖</button>
            <button class="btn" id="btn-reload" title="リロード">🔄</button>
            <a href="${targetURL}" target="_blank" rel="noopener noreferrer" class="btn" title="元のURLを新しいタブで開く">🔗</a>
        </div>
    `;

    shadow.appendChild(toolbar);

    const btnStyle = shadow.getElementById('btn-style') as HTMLButtonElement;
    const btnReload = shadow.getElementById('btn-reload') as HTMLButtonElement;
    const urlText = shadow.getElementById('url-text') as HTMLDivElement;

    urlText.addEventListener('click', () => {
        navigator.clipboard.writeText(targetURL).then(() => {
            const origText = urlText.textContent ?? '';
            urlText.textContent = 'コピーしました！';
            urlText.style.color = '#10b981';
            setTimeout(() => {
                urlText.textContent = origText;
                urlText.style.color = '';
            }, 1500);
        }).catch((err: unknown) => {
            console.error('Failed to copy: ', err);
        });
    });

    btnReload.addEventListener('click', () => {
        window.location.reload();
    });

    const storageKey = 'proxy-reader-mode';
    let isReader = localStorage.getItem(storageKey) === 'true';

    function updateStyles(): void {
        const originalStyles = document.querySelectorAll<HTMLStyleElement>('style[data-proxy-style="original"]');
        if (isReader) {
            document.body.classList.add('proxy-reader-mode');
            originalStyles.forEach(s => { s.disabled = true; });
            btnStyle.classList.add('active');
            btnStyle.title = '標準スタイルに戻す';
        } else {
            document.body.classList.remove('proxy-reader-mode');
            originalStyles.forEach(s => { s.disabled = false; });
            btnStyle.classList.remove('active');
            btnStyle.title = 'リーダーモード (CSS無効化)';
        }
    }

    btnStyle.addEventListener('click', () => {
        isReader = !isReader;
        localStorage.setItem(storageKey, isReader ? 'true' : 'false');
        updateStyles();
    });

    updateStyles();

    let lastScrollY = window.scrollY || document.documentElement.scrollTop;
    const scrollThreshold = 10;

    window.addEventListener('scroll', () => {
        const currentScrollY = window.scrollY || document.documentElement.scrollTop;

        if (currentScrollY <= 10) {
            toolbar.classList.remove('hidden');
            lastScrollY = currentScrollY;
            return;
        }

        if (Math.abs(currentScrollY - lastScrollY) < scrollThreshold) {
            return;
        }

        if (currentScrollY > lastScrollY) {
            toolbar.classList.add('hidden');
        } else {
            toolbar.classList.remove('hidden');
        }

        lastScrollY = currentScrollY;
    }, { passive: true });

    const adjustBody = (): void => {
        document.body.style.paddingTop = '48px';
    };
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', adjustBody);
    } else {
        adjustBody();
    }
})();
