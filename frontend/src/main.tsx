import React from 'react';
import { createRoot } from 'react-dom/client';
import { MantineProvider, createTheme, type MantineColorSchemeManager } from '@mantine/core';
import { QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import '@mantine/core/styles.css';
import { queryClient, router } from './app/router';
import { reportFrontendError } from './features/application/queries';
import { ThemeSync } from './usecases/switch-theme/ThemeToggle';
import './style.css';

const root = document.getElementById('root');
if (!root) throw new Error('Root element is missing');

const theme = createTheme({
  primaryColor: 'blue',
  defaultRadius: 'md',
  fontFamily: 'system-ui, sans-serif',
});

// Theme persistence is owned by settings.json (via Go service), not browser localStorage.
function memoryColorSchemeManager(): MantineColorSchemeManager {
  let current: 'light' | 'dark' | 'auto' = 'dark';
  return {
    get: (defaultValue) => current ?? defaultValue,
    set: (value) => {
      current = value;
    },
    subscribe: () => {},
    unsubscribe: () => {},
    clear: () => {
      current = 'dark';
    },
  };
}

createRoot(root, {
  onUncaughtError: reportFrontendError,
  onCaughtError: reportFrontendError,
}).render(
  <React.StrictMode>
    <MantineProvider
      theme={theme}
      defaultColorScheme="dark"
      colorSchemeManager={memoryColorSchemeManager()}
    >
      <QueryClientProvider client={queryClient}>
        <ThemeSync />
        <RouterProvider router={router} />
      </QueryClientProvider>
    </MantineProvider>
  </React.StrictMode>,
);
window.addEventListener('unhandledrejection', (event) => reportFrontendError(event.reason));
