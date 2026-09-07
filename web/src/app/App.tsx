import { RouterProvider } from 'react-router-dom';
import { appRouter } from './routes';

export function App() {
  return <RouterProvider router={appRouter} />;
}
