import styles from './toolbar.css';

(function () {
    const container = document.getElementById('proxy-toolbar-container');
    if (!container) return;

    const shadow = container.attachShadow({ mode: 'open' });

    const style = document.createElement('style');
    style.textContent = styles;
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
