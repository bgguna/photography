# 09 — Site title: "Bogdan Guna Photography", centered

## Goal

Replace the header title "Photography" with "Bogdan Guna Photography" and
center it on the page. Right now it sits on the left.

## Dependencies

None. Do this before [10](10-rename-gallery-tab.md) and
[11](11-new-page.md), which edit the same header markup.

## Scope

- `server/internal/web/templates/layout.html`: header `<h1>`, header/nav CSS,
  default `<title>` and `og:title`
- `server/internal/web/templates/contact.html`: a standalone page with its own
  copy of the header and CSS, so it needs the same changes
- `server/internal/web/templates/home.html`: the `title` and `og_tags` blocks
- `server/internal/web/templates/admin/login.html`: check its heading and
  `<title>` for consistency (optional)

## Steps

1. **Header text**: change `<h1>Photography</h1>` to
   `<h1>Bogdan Guna Photography</h1>` in `layout.html` and `contact.html`.
2. **Center it**: add `text-align: center` to `header h1` in both templates.
   Decide whether the nav links are centered too (see open question). If they
   are, add `justify-content: center` to `nav`.
3. **Leave room for the theme toggle**: the dark-mode toggle is absolutely
   positioned in the top-right of the header (`static/theme.css`). Add
   horizontal padding to the header (about 3rem each side) so the centered
   title can't run under the button on narrow phones, and keep it visually
   centered.
4. **Page titles and Open Graph tags**: update `<title>` and `og:title` to use
   the new name, for example "Bogdan Guna Photography" on the home page and
   "Contact — Bogdan Guna Photography" on the contact page.

## Open question

- Should the nav links be centered under the title, or stay left-aligned? A
  centered title over left-aligned links usually looks unbalanced, so
  centering both is the suggested default.

## Acceptance criteria

- Every public page shows "Bogdan Guna Photography", horizontally centered.
- At 360px wide, the title doesn't overlap the theme toggle and wraps cleanly
  if it has to.
- Browser tab titles and link previews use the new name.
