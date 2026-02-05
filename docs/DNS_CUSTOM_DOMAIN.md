# Step-by-step: Point docs.gdql.dev to GitHub Pages (gdql.github.io)

Use this to serve the docs site (e.g. from the **gdql/docs** repo) at **https://docs.gdql.dev** instead of (or in addition to) **https://gdql.github.io/docs**.

---

## 1. GitHub: Add the custom domain

1. Open the **docs repository** on GitHub (e.g. **github.com/gdql/docs**).
2. Go to **Settings** → **Pages** (left sidebar).
3. Under **Custom domain**, enter: **docs.gdql.dev**
4. Click **Save**.
5. (Optional) If offered, enable **Enforce HTTPS** after DNS is working.

GitHub will show a reminder that DNS is not configured yet. That’s expected; you’ll fix it in the next step.

---

## 2. DNS: Create a CNAME record at your domain registrar

Where you manage **gdql.dev** (e.g. Cloudflare, Namecheap, Google Domains, etc.):

1. Open the DNS / DNS records section for **gdql.dev**.
2. Add a **CNAME** record:

   | Type  | Name / Host | Value / Target / Points to |
   |-------|-------------|----------------------------|
   | CNAME | `docs`      | `gdql.github.io`           |

   - **Name**: `docs` (so the full hostname is **docs.gdql.dev**).  
     Some providers want **docs.gdql.dev** in the “name” field; others want only **docs**. Use whatever they use for the subdomain.
   - **Value**: **gdql.github.io** (no `https://`, no path, no trailing dot unless the provider adds it).
3. Save the record.

**Note:** If the docs site is published from a **project** repo (e.g. gdql/docs), GitHub still expects the custom domain’s CNAME to point to **gdql.github.io**. GitHub then serves the correct repo based on the domain you set in Settings → Pages.

---

## 3. Wait for DNS and GitHub

- **DNS**: Propagation usually takes a few minutes to a few hours (up to 48 hours in rare cases).
- **GitHub**: After DNS resolves, GitHub will validate the domain and (if you turned it on) provision a certificate for HTTPS. This can take a few minutes to an hour.

---

## 4. Check that it works

1. **DNS**: In a terminal, run:
   ```bash
   dig CNAME docs.gdql.dev +short
   ```
   You should see **gdql.github.io** (or a CNAME chain ending there).

2. **Site**: Open **https://docs.gdql.dev** in a browser. You should get the docs site without certificate warnings once GitHub has finished validation.

---

## 5. If the docs repo uses a baseURL with a path (e.g. `/docs/`)

If your static site (e.g. Hugo) was built with **baseURL = "https://gdql.github.io/docs/"**:

- For **docs.gdql.dev** you want the site at the **root** of the domain (so **https://docs.gdql.dev/**), not under `/docs/`.
- Update the site config so that when building for production, **baseURL** is **https://docs.gdql.dev/** (or use an environment variable so the GitHub Actions build uses this when deploying).
- Rebuild and deploy so the generated links and assets use the new base URL.

The DNS steps above are the same; only the site’s baseURL (and any canonical/redirect rules) need to match **https://docs.gdql.dev/**.

---

## Troubleshooting: “Certificate not valid for docs.gdql.dev” (SSL_ERROR_BAD_CERT_DOMAIN)

**Symptom:** Firefox (or another browser) says the certificate is only valid for `*.github.io` / `github.com`, and refuses to connect to **docs.gdql.dev** (sometimes with HSTS mentioned).

**Cause:** The server is still presenting GitHub’s default certificate instead of one that includes **docs.gdql.dev**. GitHub issues a separate certificate for your custom domain only after it can **validate** that you control that domain.

**Do this:**

1. **Confirm DNS**
   - From a terminal: `dig CNAME docs.gdql.dev +short`
   - You should see **gdql.github.io**. If you see something else (or nothing), fix the CNAME at your DNS provider so **docs** (or **docs.gdql.dev**) points to **gdql.github.io**.

2. **Confirm GitHub Pages custom domain**
   - Repo → **Settings** → **Pages**
   - **Custom domain** must be exactly **docs.gdql.dev** (no `https://` or trailing slash). Save if you changed it.

3. **Let GitHub issue the certificate**
   - After DNS is correct, GitHub validates the domain and requests a certificate (e.g. Let’s Encrypt) for **docs.gdql.dev**. This can take **a few minutes up to about 24 hours**.
   - In **Settings** → **Pages**, check for a green “DNS check successful” or “Certificate provisioned” (wording may vary). A warning like “Certificate not yet provisioned” or “DNS not configured” means wait or fix DNS.

4. **If you use Cloudflare (or another proxy) in front of gdql.dev**
   - **Option A (recommended for GitHub Pages):** Use **DNS only** (grey cloud) for the **docs** CNAME so traffic goes straight to GitHub. GitHub will then serve and certificate **docs.gdql.dev**.
   - **Option B:** If the proxy terminates HTTPS, the proxy must present a certificate that includes **docs.gdql.dev** (e.g. a Cloudflare certificate). Do not point the browser at GitHub’s origin with Host **docs.gdql.dev** while the origin still has only the `*.github.io` cert.

5. **Force re-validation (if DNS was wrong and you fixed it)**
   - In **Settings** → **Pages**, clear the **Custom domain** field and save.
   - Wait a minute, then set **Custom domain** back to **docs.gdql.dev** and save. GitHub will run validation again.

6. **HSTS**
   - The browser remembers that **docs.gdql.dev** must be loaded over HTTPS only (HSTS). You can’t “add an exception” for the bad cert. The only fix is for the **server** (GitHub or your proxy) to serve a valid certificate for **docs.gdql.dev**. Once that’s in place, the same URL will work.

**Quick checks:**

- `dig CNAME docs.gdql.dev +short` → **gdql.github.io**
- GitHub Pages **Custom domain** = **docs.gdql.dev**
- Wait for GitHub to finish certificate provisioning, then reload **https://docs.gdql.dev**.

**Debug commands (what we ran):**

```bash
dig docs.gdql.dev CNAME +short    # → gdql.github.io (DNS is correct)
curl -vI https://docs.gdql.dev   # → Server certificate: CN=*.github.io; subjectAltName does not match docs.gdql.dev
```

So: DNS points to GitHub, but GitHub is still serving its **default** cert (for `*.github.io`), not one for `docs.gdql.dev`. That means GitHub has **not finished provisioning** a certificate for the custom domain. Fix: remove the custom domain in Settings → Pages, save, wait 2–5 minutes, add **docs.gdql.dev** back and save. Check for a green “DNS check successful” / “Certificate provisioned”; it can take up to ~24 hours. CAA for gdql.dev allows Let’s Encrypt, so that’s not blocking.

---

## “Enforce HTTPS” unavailable — domain not properly configured for HTTPS

**Symptom:** In **Settings → Pages**, the **Enforce HTTPS** checkbox is greyed out and GitHub says: “Unavailable for your site because your domain is not properly configured to support HTTPS (docs.gdql.dev).”

**Cause:** GitHub can’t issue (or hasn’t yet issued) an HTTPS certificate for **docs.gdql.dev**, so it won’t let you turn on “Enforce HTTPS” until the domain is correctly set up and the cert is in place.

**Do this (same as the certificate troubleshooting above):**

1. **Check DNS**
   - `dig CNAME docs.gdql.dev +short` must return **gdql.github.io**.
   - If you use **Cloudflare** (or another proxy), set the **docs** CNAME to **DNS only** (grey cloud). Proxied traffic can prevent GitHub from validating the domain and issuing the cert.

2. **Check CAA (if you use it)**
   - GitHub uses **Let’s Encrypt** for custom domains. If you have CAA records for **gdql.dev**, they must allow Let’s Encrypt. For example, allow issuance for the right hostnames, or temporarily remove CAA for the subdomain so GitHub can get a cert. Some registrars don’t show CAA; if DNS is correct and you’re still stuck, check with your DNS provider.

3. **Give GitHub time**
   - After DNS is correct, GitHub may take from a few minutes up to about **24 hours** to validate and provision the certificate. Leave the custom domain set to **docs.gdql.dev** and check back later.

4. **Force re-check**
   - In **Settings → Pages**, clear the **Custom domain** field and save. Wait a minute, then set **docs.gdql.dev** again and save. Check whether a green “DNS check successful” (or similar) appears; once the cert is provisioned, **Enforce HTTPS** will become available and you can turn it on.

Once the domain is properly configured and the certificate is issued, the **Enforce HTTPS** option will become available and the site will load over HTTPS.

---

## Quick reference

| Step | Where | What to do |
|------|--------|------------|
| 1 | GitHub → repo **Settings** → **Pages** | Custom domain: **docs.gdql.dev** → Save |
| 2 | DNS for **gdql.dev** | CNAME **docs** → **gdql.github.io** |
| 3 | Wait | DNS + GitHub validation (minutes to ~1 hour) |
| 4 | Browser | Open **https://docs.gdql.dev** |
