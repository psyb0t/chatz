// Self-hosted webfonts. @fontsource ships pure-JS packages that import a woff2
// asset + an @font-face rule. Vite emits the woff2 into the build output and
// rewrites the url, so there is NO external font-CDN request at runtime (this
// is a self-hosted app). Import this module once from the root layout so the
// faces are present before first paint.
//
// Both are variable fonts: one file per family covers every weight app.css
// uses (see --font-sans / --font-mono).
import "@fontsource-variable/geist/index.css";
import "@fontsource-variable/geist-mono/index.css";
