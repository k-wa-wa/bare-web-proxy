import * as esbuild from 'esbuild';
import { minify as minifyHtml } from 'html-minifier-terser';
import { readFile, writeFile, mkdir } from 'fs/promises';

await mkdir('dist', { recursive: true });

// CSS imports を CSS としてミニファイしてからテキスト文字列として返すプラグイン
const cssMinifyPlugin = {
    name: 'css-minify-text',
    setup(build) {
        build.onLoad({ filter: /\.css$/ }, async (args) => {
            const source = await readFile(args.path, 'utf8');
            const { code } = await esbuild.transform(source, { loader: 'css', minify: true });
            return { contents: code, loader: 'text' };
        });
    },
};

await Promise.all([
    esbuild.build({
        entryPoints: ['toolbar.ts'],
        bundle: true,
        minify: true,
        target: 'es2020',
        outfile: 'dist/toolbar.js',
        plugins: [cssMinifyPlugin],
    }),
    esbuild.build({
        entryPoints: ['reader.css'],
        minify: true,
        outfile: 'dist/reader.css',
    }),
    readFile('index.html', 'utf8').then(src =>
        minifyHtml(src, {
            collapseWhitespace: true,
            removeComments: true,
            minifyCSS: true,
            minifyJS: true,
        })
    ).then(html => writeFile('dist/index.html', html)),
]);
