# WebUI Improvements Plan

## Phase 1: Theming & Accessibility
- [x] **Styles**: Add `sepia` (warm/reading) and `high-contrast` (accessibility) classes to `frontend/src/index.css`.
- [x] **Switcher**: Update `ThemeSwitcher` in `frontend/src/components/Layout.jsx` to support the new modes.

## Phase 2: Dashboard (Mobile Optimization)
- [x] **StatCards Carousel**: Modify `frontend/src/pages/Dashboard.jsx` to use a horizontal scroll container (carousel) for `StatCards` on mobile instead of a grid.
- [x] **System Resources**: Make `SystemResources` component in `frontend/src/pages/Dashboard.jsx` collapsible (accordion style) on mobile to save vertical space.
- [x] **Empty States**: Improve empty state in `Dashboard.jsx` with clearer actions.

- [x] **Mobile Navigation**: Refactor `frontend/src/pages/IssuesEditor.jsx` to use a "Slide Over" pattern (List -> Editor) instead of top tabs.
- [x] **Create Issue FAB**: Add a Floating Action Button (FAB) for creating issues on mobile in `IssuesEditor.jsx`.
- [x] **Desktop Button**: Enhance visibility of the "Create Issue" button in Desktop view (likely in `IssuesSidebar` or `IssuesActionBar`).
## Phase 4: Settings (Organization)
- [ ] **Grouping**: Group Settings tabs in `frontend/src/pages/Settings.jsx` into "System" and "Project" categories.
- [x] **Grouping**: Group Settings tabs in `frontend/src/pages/Settings.jsx` into "System" and "Project" categories.
- [x] **Mobile Transitions**: Add slide animations (`animate-slide-in`) for mobile menu navigation.
- [x] **Search**: Optimize search bar for mobile (collapsible or better spacing).
