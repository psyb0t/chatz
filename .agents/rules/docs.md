# Docs and releases

- The documentation site is `docs/`, published by GitHub Pages from `main` at https://psyb0t.github.io/chatz/ with the Just the Docs theme (`remote_theme` pinned in `docs/_config.yml`, colors in `docs/_sass/`, the shared visitor tracker in `docs/_includes/head_custom.html`).
- Every page has front matter with a unique `title` and a `description` of 50 to 170 characters, and exactly one `#` heading. Child pages set `parent` to the parent's exact title.
- Links between pages use the relative `.md` path. A link to a file outside `docs/` uses the full `https://github.com/psyb0t/chatz/blob/main/<path>` URL, or `tree/main/<path>` for a directory. Wrap code blocks containing `{{` or `{%` in `<!-- {% raw %} -->` and `<!-- {% endraw %} -->`.
- After a docs change, run `make docs` and check the built site in `.testing/docs-site` before handing off.
- The README is the front door: what chatz is, the quick start, and a Documentation section with full site URLs. Reference material goes in `docs/`; module READMEs keep the developer deep dives. Each fact has one home.
- A behavior, setting, endpoint or command change updates the matching docs page and `.env.example` in the same change.
- Prose: no em dashes, "directory" never "folder", no AI vocabulary, sentence case headings, one paragraph per source line. Do not mention local scratch directories in committed text.
- Releases: a `v<semver>` tag on a commit whose `main` pipeline run passed. The pipeline promotes that run's `sha-<commit>` image to the tag and cuts the GitHub release. CHANGELOG entries are written at release time.
