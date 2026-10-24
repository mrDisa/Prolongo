import { createBrowserRouter } from 'react-router-dom';
import { RequireAuth } from '@/components/RequireAuth';
import { AppLayout } from '@/layouts/AppLayout';
import { AuthLayout } from '@/layouts/AuthLayout';
import Customer from '@/pages/Customer';
import Customers from '@/pages/Customers';
import Data from '@/pages/Data';
import Login from '@/pages/Login';
import Model from '@/pages/Model';
import NotFound from '@/pages/NotFound';
import Overview from '@/pages/Overview';

export const router = createBrowserRouter([
  {
    element: <AuthLayout />,
    children: [{ path: '/login', element: <Login /> }],
  },
  {
    // Everything below needs a session.
    element: <RequireAuth />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { path: '/', element: <Overview /> },
          { path: '/customers', element: <Customers /> },
          { path: '/customers/:id', element: <Customer /> },
          { path: '/model', element: <Model /> },
          { path: '/data', element: <Data /> },
          { path: '*', element: <NotFound /> },
        ],
      },
    ],
  },
]);
