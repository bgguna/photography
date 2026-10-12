# 13 — Evaluate: Cloudflare protection for the whole site

## Goal

Decide whether the whole site, not just the contact form, should sit behind
a Cloudflare bot check. This is an **investigation/decision task**: the output
is a recorded decision, plus a follow-up implementation task if the answer is
yes.

## How it works (summary for the decision)

There are two different Cloudflare features, and they behave differently.

**1. Turnstile (the widget from [12](12-contact-turnstile.md))**
- Protects a single action, such as a form submit. The token is single-use
  and expires after 5 minutes.
- It is **not session-based**. Gating every page with it would mean building
  our own "verified" session cookie on top. That's possible, but it's the
  wrong tool for this job.

**2. Cloudflare proxy + challenges (site-wide)**
- Traffic goes through Cloudflare first, either by moving DNS to Cloudflare
  with the proxy enabled (orange cloud) or with a **Cloudflare Tunnel** from
  the Pi. A tunnel also avoids opening ports 80/443 on the home router.
- You can then enable, without any code changes:
  - **Bot Fight Mode**: blocks known bots
  - **WAF custom rules** with a **Managed Challenge**, for example only on
    `/admin/*`, only for certain countries, or for everything
  - **"I'm Under Attack" mode**: challenges every visitor, meant for active
    attacks only
- **This one *is* session-like.** A visitor who passes gets a `cf_clearance`
  cookie and isn't challenged again until it expires. The lifetime is the
  zone's "Challenge Passage" setting, which defaults to 30 minutes and can be
  changed.

## Trade-offs

For:
- DDoS protection and edge caching of images, which reduces load on the Pi.
- Hides the home IP address.
- With a Tunnel there are no open inbound ports.

Against:
- Challenging *every* visitor adds friction to a public portfolio whose job
  is to be looked at.
- Challenges can block link-preview crawlers (Open Graph cards in messaging
  apps and social media) and search engines unless they are allowed through
  explicitly.
- Changes the deployment in [08](08-deployment-pi.md): Caddy's role (TLS),
  DNS, and Gin's trusted proxies (real client IP comes from
  `CF-Connecting-IP`).
- Adds a dependency on Cloudflare being up.

## Suggested direction (to confirm)

- **Don't** put a challenge on every page.
- **Do** consider the Cloudflare proxy or a Tunnel for the network benefits:
  DDoS protection, caching, hidden IP, no open ports. Optionally add Bot Fight
  Mode and a Managed Challenge rule scoped to `/admin/*`.
- Keep Turnstile on the contact form (task 12), and optionally on the admin
  login form.

## Steps

1. Confirm how the site is exposed today: a public domain, or LAN/Tailscale
   only. If it isn't public, most of this doesn't apply.
2. Check the facts above against current Cloudflare docs: plan limits on the
   free tier, Challenge Passage settings, Tunnel setup on ARM.
3. Record the decision in `docs/design.md` (deployment section).
4. If the answer is yes, write a follow-up task covering DNS or Tunnel setup,
   Caddy changes, trusted proxies, and WAF rules.

## Acceptance criteria

- A recorded yes or no, with the reasoning, in `docs/design.md`.
- If yes, a follow-up implementation task exists in `docs/tasks/`.
