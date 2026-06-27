import styles from './toolbar.css';

const HAMBURGER_SVG = `<svg width="14" height="10" viewBox="0 0 14 10" fill="currentColor"><rect y="0" width="14" height="1.5" rx="0.75"/><rect y="4.25" width="14" height="1.5" rx="0.75"/><rect y="8.5" width="14" height="1.5" rx="0.75"/></svg>`;
const READER_KEY = 'proxy-reader-mode';
const ISSUE_URL_BASE = 'https://github.com/k-wa-wa/bare-web-proxy/issues/new';

const targetURL = (window as Window & { __PROXY_TARGET_URL__?: string }).__PROXY_TARGET_URL__ ?? '';

function buildIssueURL(): string {
    let domain = '';
    try { domain = new URL(targetURL).hostname; } catch { domain = targetURL; }
    return ISSUE_URL_BASE
        + '?title=' + encodeURIComponent(`${domain}の表示崩れ改善`)
        + '&body=' + encodeURIComponent(targetURL);
}

function createShadow(container: Element): ShadowRoot {
    const shadow = container.attachShadow({ mode: 'open' });
    const styleEl = document.createElement('style');
    styleEl.textContent = styles;
    shadow.appendChild(styleEl);
    return shadow;
}

function createToolbar(shadow: ShadowRoot): HTMLDivElement {
    const toolbar = document.createElement('div');
    toolbar.className = 'toolbar';
    toolbar.innerHTML = `
        <div class="left-section">
            <a href="/" target="_self" class="brand" title="ホームへ戻る">🐻 proxy</a>
            <div class="url-info" id="url-text" title="${targetURL}">${targetURL}</div>
        </div>
        <div class="right-section">
            <button class="btn" id="btn-style" title="リーダーモード (CSS無効化)">📖</button>
            <button class="btn" id="btn-reload" title="リロード">🔄</button>
            <div class="menu-container">
                <button class="btn" id="btn-menu" title="メニュー">${HAMBURGER_SVG}</button>
                <div class="menu-dropdown" id="menu-dropdown">
                    <a href="${targetURL}" target="_blank" rel="noopener noreferrer" class="menu-item">
                        <span class="menu-icon">🔗</span>ソースを別タブで開く
                    </a>
                    <a href="${buildIssueURL()}" target="_blank" rel="noopener noreferrer" class="menu-item">
                        <span class="menu-icon">🐛</span>issueを報告する
                    </a>
                </div>
            </div>
        </div>
    `;
    shadow.appendChild(toolbar);
    return toolbar;
}

function setupURLCopy(shadow: ShadowRoot): void {
    const urlText = shadow.getElementById('url-text') as HTMLDivElement;
    urlText.addEventListener('click', () => {
        navigator.clipboard.writeText(targetURL).then(() => {
            const orig = urlText.textContent ?? '';
            urlText.textContent = 'コピーしました！';
            urlText.style.color = '#10b981';
            setTimeout(() => { urlText.textContent = orig; urlText.style.color = ''; }, 1500);
        }).catch((err: unknown) => console.error('clipboard write failed:', err));
    });
}

function setupReaderMode(shadow: ShadowRoot): void {
    const btnStyle = shadow.getElementById('btn-style') as HTMLButtonElement;
    const btnReload = shadow.getElementById('btn-reload') as HTMLButtonElement;
    let isReader = localStorage.getItem(READER_KEY) === 'true';

    const apply = (): void => {
        const origStyles = document.querySelectorAll<HTMLStyleElement>('style[data-proxy-style="original"]');
        document.body.classList.toggle('proxy-reader-mode', isReader);
        origStyles.forEach(s => { s.disabled = isReader; });
        btnStyle.classList.toggle('active', isReader);
        btnStyle.title = isReader ? '標準スタイルに戻す' : 'リーダーモード (CSS無効化)';
    };

    btnStyle.addEventListener('click', () => {
        isReader = !isReader;
        localStorage.setItem(READER_KEY, String(isReader));
        apply();
    });
    btnReload.addEventListener('click', () => window.location.reload());

    apply();
}

function setupMenu(shadow: ShadowRoot): () => void {
    const btnMenu = shadow.getElementById('btn-menu') as HTMLButtonElement;
    const dropdown = shadow.getElementById('menu-dropdown') as HTMLDivElement;

    const close = (): void => {
        dropdown.classList.remove('open');
        btnMenu.classList.remove('active');
    };

    btnMenu.addEventListener('click', (e) => {
        e.stopPropagation();
        const open = dropdown.classList.toggle('open');
        btnMenu.classList.toggle('active', open);
    });
    document.addEventListener('click', close);

    return close;
}

function setupScrollBehavior(toolbar: HTMLDivElement, closeMenu: () => void): void {
    let lastY = window.scrollY || document.documentElement.scrollTop;
    const threshold = 10;

    window.addEventListener('scroll', () => {
        const y = window.scrollY || document.documentElement.scrollTop;
        closeMenu();

        if (y <= 10) {
            toolbar.classList.remove('hidden');
            lastY = y;
            return;
        }
        if (Math.abs(y - lastY) < threshold) return;
        toolbar.classList.toggle('hidden', y > lastY);
        lastY = y;
    }, { passive: true });
}

function adjustBodyPadding(): void {
    const apply = (): void => { document.body.style.paddingTop = '48px'; };
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', apply);
    } else {
        apply();
    }
}

function init(): void {
    const container = document.getElementById('proxy-toolbar-container');
    if (!container) return;

    const shadow = createShadow(container);
    const toolbar = createToolbar(shadow);

    setupURLCopy(shadow);
    setupReaderMode(shadow);
    const closeMenu = setupMenu(shadow);
    setupScrollBehavior(toolbar, closeMenu);
    adjustBodyPadding();
}

init();
