# Notes for the docs site (docs.gdql.dev)

If the language reference and other docs are rendered as a static site at **docs.gdql.dev** (e.g. from a separate docs repo or Hugo/VitePress/Docusaurus build), use these notes when maintaining that site.

---

## Mobile menu: avoid blur that obscures content

**Problem:** A mobile nav overlay that uses `backdrop-filter: blur(...)` (or similar) can make the entire page content unreadable behind the menu. Users report the menu “blurs everything” and the site is unusable on mobile.

**Fix:** Use a **solid background** for the mobile menu overlay instead of a blurred glass effect:

- **Option A:** Set the overlay to an opaque background matching the theme (e.g. `background: var(--page-background)` or your theme’s surface color). Remove or disable `backdrop-filter` and any semi-transparent background on the overlay.
- **Option B:** If you keep a translucent overlay, do **not** use `backdrop-filter: blur()` on the main content area; use a solid panel for the menu drawer so only the menu panel is visible, not a blurred version of the page behind it.

After the change, the mobile menu should remain readable and the rest of the page should not be blurred when the menu is open.

---

## Linking to the sandbox

The [LANGUAGE.md](LANGUAGE.md) reference uses **Try in Sandbox** links that point to `https://sandbox.gdql.dev?example=<slug>&run=1`. Keep that base URL when copying or building the docs so each example opens a prepopulated query in the sandbox.
