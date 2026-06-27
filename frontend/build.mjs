import * as esbuild from 'esbuild';
import { minify as minifyHtml } from 'html-minifier-terser';
import { readFile, writeFile, mkdir } from 'fs/promises';

const outDir = 'dist';

await mkdir(outDir, { recursive: true });

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
        entryPoints: ['src/toolbar.ts'],
        bundle: true,
        minify: true,
        target: 'es2020',
        outfile: `${outDir}/toolbar.js`,
        plugins: [cssMinifyPlugin],
    }),
    esbuild.build({
        entryPoints: ['src/reader.css'],
        minify: true,
        outfile: `${outDir}/reader.css`,
    }),
    readFile('src/index.html', 'utf8').then(src =>
        minifyHtml(src, {
            collapseWhitespace: true,
            removeComments: true,
            minifyCSS: true,
            minifyJS: true,
        })
    ).then(html => writeFile(`${outDir}/index.html`, html)),
]);
