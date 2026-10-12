# 14 — More space between "Get in Touch" and the form

## Goal

Add vertical space between the "Get in Touch" heading and the first field
("Name *") on the contact page.

## Cause

The global `* { margin: 0; padding: 0; }` reset in `contact.html` removes the
heading's default margin, and nothing adds it back. The `<h2>` sits directly
on top of the form.

## Dependencies

None. Coordinate with [12](12-contact-turnstile.md) and
[15](15-contact-email-required.md), which also edit `contact.html`.

## Scope

- `server/internal/web/templates/contact.html` (CSS only)

## Steps

1. Add a `margin-bottom` to the page heading (`main h2`), around `1.5rem`–
   `2rem`. That matches the existing `.form-group` spacing of `1.5rem`.
2. Check the spacing with the success and error banners showing, since they
   sit between the heading and the form. The heading-to-banner and
   banner-to-form gaps should look consistent.
3. If task 11 adds a new page, use the same heading style there so page
   headings match across the site.

## Acceptance criteria

- There is visible breathing room between "Get in Touch" and "Name *" on
  desktop and at phone width.
- The success and error banners don't end up with doubled or uneven spacing.
