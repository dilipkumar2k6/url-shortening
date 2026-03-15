# Frontend Prompt

Generate a modern, high-performance URL shortener dashboard.

## Core Requirements

1. **URL Shortening Interface**:
   - A clean, centered input field for long URLs.
   - A prominent "Shorten" button.
   - Display the generated short URL with a "Copy" button.
   - Support for dynamic base URLs (configured via environment variables).

2. **Real-time Analytics Dashboard**:
   - A table showing the top 10 performing links.
   - Columns: **Short Link**, **Destination (Long URL)**, and **Clicks**.
   - **Real-time Updates**: Implement a 5-second refetch interval using **React Query** to ensure the dashboard stays up-to-date.
   - **Truncation**: Truncate long destination URLs with an ellipsis for better layout, but show the full URL on hover (title attribute).

3. **Design Aesthetics**:
   - Use a clean, modern aesthetic (e.g., white background with blue accents).
   - Use **Tailwind CSS** for styling.
   - Use **Shadcn UI** components for a premium feel (Card, Table, Button, Input).
   - Add subtle hover effects and micro-animations.

4. **Responsiveness**:
   - The UI must be fully responsive and look great on mobile, tablet, and desktop.

5. **Testing**:
   - Provide **Playwright** integration tests covering the full flow: shorten a URL, click it, and verify it appears in the analytics table.

## Technology Stack
- **Framework**: React (Vite)
- **Styling**: Tailwind CSS
- **UI Components**: Shadcn UI
- **Icons**: Lucide React
- **Data Fetching**: TanStack Query (React Query)
- **Testing**: Playwright
