# Web experience

YuFlix uses React, TypeScript, Vite, React Router declarative routing, and the YuTheme color system.

## Navigation and authentication

- /login is the public dedicated sign-in screen.
- / is the signed-in library home.
- /admin is available only to users with the admin role.
- Browser sessions are restored using the authenticated /api/v1/auth/me endpoint.
- The frontend never stores passwords or session tokens in localStorage or JavaScript-accessible cookies. Authentication remains on server-managed HttpOnly cookies.
- HTTP and HTTPS use the installation's explicitly configured allowed origins and separate session cookies.
- The login screen warns users about the security limitations of HTTP.

## Product truth

There are no fabricated movie posters, pretend playable controls, fake libraries, or simulated streams. The signed-in home explains that media discovery and playback are not ready. Once media is implemented, catalog discovery and playback must be backed by the real API before controls become active.

## Visual standards

- Import YuTheme tokens exclusively for color decisions. AMOLED theme is the UI baseline.
- Use locally bundled DM Sans Variable for an editorial, cinema-inspired identity.
- Do not load Google Fonts, external image CDNs, or remote CSS at runtime.
- Screen reader headings, semantic forms, visible focus, adaptive mobile layout and reduced-motion support are required.
- Administration is visually and functionally separate from personal browsing.

## Quality

Run pnpm test and pnpm build from the repo root. Component tests cover access guards, invalid credentials, password visibility, successful navigation, role-based access control and disabled sign-in when the origin is not authorized. These checks are not a substitute for actual browser visual review, which must be completed before claiming premium UI validation.

## Apple TV-inspired direction

The aesthetic is based on the general content-first approach described by Apple's Human Interface Guidelines for tvOS, not an Apple-branded clone. The reference emphasizes immersive layouts, clear navigation/focus, readable typography, restrained materials and straightforward multi-user access. The application uses its own YuFlix branding and only YuTheme color tokens.

The login is a full-page, cardless layout with one large, prominent sign-in form. The home uses a floating, focus-visible navigation system and reserves cinematic hero space for actual media art. The admin uses quiet, spacious settings-like lists and never impersonates a streaming screen. Motion honors reduced-motion settings.

Official references:

- https://developer.apple.com/design/human-interface-guidelines/designing-for-tvos
- https://developer.apple.com/design/human-interface-guidelines/materials

Component CSS is scoped by context in apps/web/src/styles/{foundation,login,home,admin}.css.

Automated headless Chromium checks covered 390px mobile, 820px tablet, 1440px desktop and 1920px TV viewports. These checked layout overflow, console/runtime errors, login focus and administration navigation with stubbed account data. Browser-only checks do not replace an actual user review on the installed device.
