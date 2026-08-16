// Self-hosted webfonts. @fontsource ships pure-JS packages that import a woff2
// asset + an @font-face rule per weight — Vite emits the woff2 into the build
// output and rewrites the url, so there is NO external font-CDN request at
// runtime (this is a self-hosted app). Import this module once from the root
// layout so the faces are present before first paint.
//
// Weights match the design system's usage: 400 for body/mono text, 700 for the
// brutalist display headings/labels (see app.css --font-display / --font-mono).
import "@fontsource/space-grotesk/400.css";
import "@fontsource/space-grotesk/700.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/700.css";
