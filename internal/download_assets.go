//go:build generate

package depproxy

//go:generate curl --silent --show-error --output assets/highlightjs.css https://cdn.jsdelivr.net/gh/highlightjs/cdn-release/build/styles/default.min.css
//go:generate curl --silent --show-error --output assets/diff2html.css https://cdn.jsdelivr.net/npm/diff2html/bundles/css/diff2html.min.css
//go:generate curl --silent --show-error --output assets/diff2html.js https://cdn.jsdelivr.net/npm/diff2html/bundles/js/diff2html-ui.min.js
