package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	// ポート3003で起動
	port := "3003"

	// Simple Test Page
	http.HandleFunc("/simple", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Simple Test</title></head>
<body>
    <h1>Simple Page</h1>
    <p>Minimal content without styling or scripting.</p>
</body>
</html>`))
	})

	// Standard CSS Test Page
	http.HandleFunc("/css-standard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Standard CSS Test</title>
    <link rel="stylesheet" href="/static/style1.css">
    <link rel="stylesheet" href="/static/style2.css">
</head>
<body>
    <div class="box1">
        <h1>Standard CSS Page</h1>
    </div>
    <div class="box2">
        <p>Styled with multiple external CSS files.</p>
    </div>
</body>
</html>`))
	})

	// Massive CSS Test Page
	http.HandleFunc("/css-massive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Massive CSS Test</title>
    <link rel="stylesheet" href="/static/style-massive.css">
</head>
<body>
    <h1>Massive CSS Page</h1>
    <div class="test-rule-1">Styled page with heavy rules.</div>
</body>
</html>`))
	})

	// Dynamic SPA Test Page
	http.HandleFunc("/spa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Dynamic SPA Test</title>
    <style>
        .dynamic-box { padding: 10px; margin: 5px; border-radius: 4px; }
    </style>
</head>
<body>
    <h1>Dynamic SPA Page</h1>
    <div id="app">Loading...</div>
    <script>
        setTimeout(() => {
            const app = document.getElementById('app');
            app.innerHTML = '<div class="dynamic-box" style="background: lightblue;">Loaded step 1 (1s)</div>';
        }, 1000);
        setTimeout(() => {
            const app = document.getElementById('app');
            app.innerHTML += '<div class="dynamic-box" style="background: lightgreen;">Loaded step 2 (2s)</div>';
        }, 2000);
        setTimeout(() => {
            const app = document.getElementById('app');
            app.innerHTML += '<div class="dynamic-box" style="background: coral;">Final step loaded! (3s)</div>';
            document.body.classList.add('ready');
        }, 3000);
    </script>
</body>
</html>`))
	})

	// Slow Loading Test Page
	http.HandleFunc("/slow-loading", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Slow CSS Test</title>
    <link rel="stylesheet" href="/static/style-slow.css">
</head>
<body>
    <div class="slow-box">
        <h1>Slow Loading CSS Page</h1>
    </div>
</body>
</html>`))
	})

	// Static Assets Server
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasSuffix(path, "style1.css") {
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte(`body { font-family: sans-serif; background-color: #fafafa; }
.box1 { padding: 20px; border-bottom: 2px solid #ccc; }`))
			return
		}

		if strings.HasSuffix(path, "style2.css") {
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte(`.box2 { color: #333; line-height: 1.6; font-size: 1.1em; }`))
			return
		}

		if strings.HasSuffix(path, "style-massive.css") {
			w.Header().Set("Content-Type", "text/css")
			// 約100KBのダミーCSSを動的に生成
			var sb strings.Builder
			sb.WriteString("/* Massive CSS File */\n")
			for i := 1; i <= 2000; i++ {
				fmt.Fprintf(&sb, ".dummy-class-selector-%d { color: rgb(%d, %d, %d); margin: %dpx; padding: %dpx; }\n",
					i, i%256, (i*2)%256, (i*3)%256, i%20, i%10)
			}
			w.Write([]byte(sb.String()))
			return
		}

		if strings.HasSuffix(path, "style-slow.css") {
			// 2秒間遅延させる
			time.Sleep(2 * time.Second)
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte(`.slow-box { background-color: lightgoldenrodyellow; border: 1px dashed orange; padding: 20px; }`))
			return
		}

		http.NotFound(w, r)
	})

	log.Printf("Mock Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
