# Docs and releases

- The documentation site is `docs/`, published by GitHub Pages from `main` at https://psyb0t.github.io/chatz/ with the Just the Docs theme (`remote_theme` pinned in `docs/_config.yml`, colors in `docs/_sass/`, the shared visitor tracker in `docs/_includes/head_custom.html`).
- Child pages set `parent` to the parent's exact title.
- Wrap code blocks containing `{{` or `{%` in `<!-- {% raw %} -->` and `<!-- {% endraw %} -->`, or Jekyll renders them empty.
- After a docs change, run `make docs` and check the built site in `.testing/docs-site` before handing off.
- Module READMEs keep the developer deep dives; user reference material goes in `docs/`.
- A behavior, setting, endpoint or command change updates the matching docs page and `.env.example` in the same change.
- Releases: a `v<semver>` tag on a commit whose `main` pipeline run passed. The pipeline promotes that run's `sha-<commit>` image to the tag and cuts the GitHub release.
