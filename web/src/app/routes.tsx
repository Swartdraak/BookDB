import { createBrowserRouter, type RouteObject } from 'react-router-dom';
import { AppShell } from './layouts/AppShell';
import { NotFoundPage } from './pages/NotFoundPage';
import { OverviewPage } from './pages/OverviewPage';
import { SearchPage } from './pages/SearchPage';
import { SettingsPage } from './pages/SettingsPage';
import { WorkDetailPage } from './pages/WorkDetailPage';
import { WorkspacePage } from './pages/WorkspacePage';

export const appRoutes: RouteObject[] = [
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <OverviewPage /> },
      { path: 'search', element: <SearchPage /> },
      { path: 'works/:id', element: <WorkDetailPage /> },
      { path: 'workspace', element: <WorkspacePage /> },
      { path: 'settings', element: <SettingsPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
];

export const appRouter = createBrowserRouter(appRoutes);
