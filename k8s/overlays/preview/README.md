# preview 環境

open な PR 1本につき 1環境を `bare-web-proxy-pr-<N>` namespace に自動で立ち上げる。
PR を立てれば生え、close/merge すれば namespace ごと消える。

- URL: `https://bwproxy-pr-<N>.wpcapp.net`

![prod / preview 比較](./preview-vs-prod.drawio.svg)
