(function() {
    const container = document.getElementById('proxy-toolbar-container');
    if (!container) return;
    
    const shadow = container.attachShadow({mode: 'open'});
    
    // Shadow DOM 内の CSS (ガラスモーフィズム、モバイルフレンドリー、統一された丸型ボタンデザイン)
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
    
    const targetURL = window.__PROXY_TARGET_URL__ || '';
    
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
    
    const btnStyle = shadow.getElementById('btn-style');
    const btnReload = shadow.getElementById('btn-reload');
    const urlText = shadow.getElementById('url-text');
    
    // URLのコピー機能
    urlText.addEventListener('click', () => {
        navigator.clipboard.writeText(targetURL).then(() => {
            const origText = urlText.textContent;
            urlText.textContent = "コピーしました！";
            urlText.style.color = "#10b981";
            setTimeout(() => {
                urlText.textContent = origText;
                urlText.style.color = "";
            }, 1500);
        }).catch(err => {
            console.error('Failed to copy: ', err);
        });
    });
    
    // リロード処理
    btnReload.addEventListener('click', () => {
        window.location.reload();
    });
    
    // スタイル切り替え処理
    const storageKey = 'proxy-reader-mode';
    let isReader = localStorage.getItem(storageKey) === 'true';
    
    function updateStyles() {
        const originalStyles = document.querySelectorAll('style[data-proxy-style="original"]');
        if (isReader) {
            document.body.classList.add('proxy-reader-mode');
            originalStyles.forEach(s => s.disabled = true);
            btnStyle.classList.add('active');
            btnStyle.title = '標準スタイルに戻す';
        } else {
            document.body.classList.remove('proxy-reader-mode');
            originalStyles.forEach(s => s.disabled = false);
            btnStyle.classList.remove('active');
            btnStyle.title = 'リーダーモード (CSS無効化)';
        }
    }
    
    btnStyle.addEventListener('click', () => {
        isReader = !isReader;
        localStorage.setItem(storageKey, isReader ? 'true' : 'false');
        updateStyles();
    });
    
    // 初回適用
    updateStyles();
    
    // スクロール時のツールバー表示・非表示制御
    let lastScrollY = window.scrollY || document.documentElement.scrollTop;
    const scrollThreshold = 10; // 判定用のスクロール量閾値(px)
    
    window.addEventListener('scroll', () => {
        const currentScrollY = window.scrollY || document.documentElement.scrollTop;
        
        // 最上部に近い場合は常に表示
        if (currentScrollY <= 10) {
            toolbar.classList.remove('hidden');
            lastScrollY = currentScrollY;
            return;
        }
        
        // 閾値以上のスクロール移動があった場合のみ判定
        if (Math.abs(currentScrollY - lastScrollY) < scrollThreshold) {
            return;
        }
        
        if (currentScrollY > lastScrollY) {
            // 下スクロール -> 隠す
            toolbar.classList.add('hidden');
        } else {
            // 上スクロール -> 見せる
            toolbar.classList.remove('hidden');
        }
        
        lastScrollY = currentScrollY;
    }, { passive: true });
    
    // ボディのパディング調整
    const adjustBody = () => {
        document.body.style.paddingTop = '48px';
    };
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', adjustBody);
    } else {
        adjustBody();
    }
})();
