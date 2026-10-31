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

There are no fabricated movie posters, pretend playable controls, fake libraries, or simulated streams. The signed-in home displays only real Stremio metadata. It offers an experimental Direct Play MP4 flow, but title presence and source discovery never establish playback compatibility; no fake streams or posters are shown.

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

The login is a full-page, cardless layout with one large, prominent sign-in form. The shared authenticated shell keeps the navigation and account controls stable across Home, title details and Administration. Route content uses React 19.3 ViewTransition (with React Router 8) with restrained cross-fade and vertical offset; admin subsections and catalog filters have their own short transitions. Browsers without View Transitions remain fully functional, and prefers-reduced-motion disables these animations.

The library has data-backed Movies/Series/All filters, keyboard-focusable rails and explicit previous/next actions. Administration sections are addressable with /admin?section=users|preferences|catalogs|streams and support browser history. The authenticated source list opens an immersive, focus-managed HTML5 video dialog, with Escape/close, fullscreen where available and clearly labeled buffering and error states. Only backend-issued same-origin media tickets may be played. The modal does not claim universal codec compatibility or guaranteed playback.

Official references:

- https://developer.apple.com/design/human-interface-guidelines/designing-for-tvos
- https://developer.apple.com/design/human-interface-guidelines/materials

Component CSS is split between existing context styles and apps/web/src/styles/premium.css. All colors remain YuTheme token references.

Automated headless Chromium checks covered 390px mobile, 820px tablet, 1440px desktop and 1920px TV viewports. These checked layout overflow, console/runtime errors, login focus and administration navigation with stubbed account data. Browser-only checks do not replace an actual user review on the installed device.

On 2026-10-31, an isolated, ephemeral Playwright Chromium headless shell on LAB verified navigation, category filter, title detail, source dialog, administration and horizontal overflow at 390px, 820px, 1440px and 1920px, with mocked safe media API responses. No console/runtime errors remained other than a deliberately mocked 404 media response. This does not prove real provider playback or physical device compatibility. The temporary Vite server was stopped after the checks.

Series episode selection precedes source discovery and uses a bounded, scrollable episode grid to avoid forcing a long scroll past every episode.
