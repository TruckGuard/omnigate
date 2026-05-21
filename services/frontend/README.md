# 💻 TruckGuard Frontend

### 1. What is it?

The **Frontend Service** is the web dashboard for the TruckGuard ecosystem. Built with **SvelteKit**, it provides real-time visualization of gate operations, active transactions, event logs, and mapping configurations.

### 2. Purpose & Features

- **Dashboard & KPIs**:
  - Live metric visualization (active permits, event counters, ANPR plate matching status).
- **Interactive Mapping Editor**:
  - Direct UI for editing device configurations, setting payload JSONPath mappings, and previewing transformed event payloads.
- **Transaction Logs**:
  - Real-time event streams and transaction sticky session tracking.
- **Administration Panels**:
  - Manage Gates, Event Types, User Profiles, and Auth Roles.

### 3. Tech Stack

- **Framework**: [SvelteKit](https://kit.svelte.dev/)
- **Build Tool**: Vite
- **Styling**: Vanilla CSS (sleek dark mode, custom UI elements)

### 4. Getting Started

#### **Prerequisites**

- Node.js (v18 or higher)
- npm or pnpm

#### **Run Commands**

1.  **Install dependencies:**
    ```bash
    npm install
    ```
2.  **Start development server:**
    ```bash
    npm run dev
    ```

### 5. Configuration (Environment Variables)

```env
PUBLIC_API_URL=http://localhost:8090
```
