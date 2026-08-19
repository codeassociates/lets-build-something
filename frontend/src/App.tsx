import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import type { ReactNode } from 'react'
import { Layout, Page } from './components/Layout'
import { Spinner } from './components/ui'
import { useAuth } from './auth'
import type { Role } from './api/types'

import { Catalog } from './pages/shop/Catalog'
import { ItemDetail } from './pages/shop/ItemDetail'
import { Checkout } from './pages/shop/Checkout'
import { SignIn } from './pages/SignIn'
import { Register } from './pages/Register'
import { MyRentals } from './pages/account/MyRentals'
import { ReservationDetail } from './pages/account/ReservationDetail'
import { MyInvoices } from './pages/account/MyInvoices'
import { Profile } from './pages/account/Profile'
import { DeskToday } from './pages/desk/DeskToday'
import { DeskReservation } from './pages/desk/DeskReservation'
import { DeskCustomers } from './pages/desk/DeskCustomers'
import { AdminDashboard } from './pages/admin/AdminDashboard'
import { AdminItems } from './pages/admin/AdminItems'
import { AdminFleet } from './pages/admin/AdminFleet'
import { AdminUsers } from './pages/admin/AdminUsers'
import { AdminEmails } from './pages/admin/AdminEmails'
import { AdminJobs } from './pages/admin/AdminJobs'

export function App() {
  return (
    <Layout>
      <Routes>
        {/* shop — open to anyone, so people can browse before signing up */}
        <Route path="/" element={<Catalog />} />
        <Route path="/items/:id" element={<ItemDetail />} />
        <Route path="/checkout" element={<Checkout />} />
        <Route path="/signin" element={<SignIn />} />
        <Route path="/register" element={<Register />} />

        {/* customer */}
        <Route path="/account" element={<Guard><MyRentals /></Guard>} />
        <Route path="/account/reservations/:id" element={<Guard><ReservationDetail /></Guard>} />
        <Route path="/account/invoices" element={<Guard><MyInvoices /></Guard>} />
        <Route path="/account/profile" element={<Guard><Profile /></Guard>} />

        {/* service desk */}
        <Route path="/desk" element={<Guard role="staff"><DeskToday /></Guard>} />
        <Route path="/desk/reservations/:id" element={<Guard role="staff"><DeskReservation /></Guard>} />
        <Route path="/desk/customers" element={<Guard role="staff"><DeskCustomers /></Guard>} />

        {/* administration */}
        <Route path="/admin" element={<Guard role="staff"><AdminDashboard /></Guard>} />
        <Route path="/admin/items" element={<Guard role="staff"><AdminItems /></Guard>} />
        <Route path="/admin/fleet" element={<Guard role="staff"><AdminFleet /></Guard>} />
        <Route path="/admin/users" element={<Guard role="admin"><AdminUsers /></Guard>} />
        <Route path="/admin/emails" element={<Guard role="staff"><AdminEmails /></Guard>} />
        <Route path="/admin/jobs" element={<Guard role="admin"><AdminJobs /></Guard>} />

        <Route path="*" element={<NotFound />} />
      </Routes>
    </Layout>
  )
}

/**
 * Gate for a route. Sends an anonymous visitor to sign in, remembering where
 * they were headed; tells a signed-in user plainly when their role is not
 * enough, rather than bouncing them somewhere confusing.
 */
function Guard({ role = 'customer', children }: { role?: Role; children: ReactNode }) {
  const { user, loading, can } = useAuth()
  const location = useLocation()

  if (loading) return <Page><Spinner /></Page>
  if (!user) return <Navigate to="/signin" replace state={{ from: location.pathname }} />
  if (!can(role)) {
    return (
      <Page narrow>
        <div className="note note-warn">
          <strong>Not available to your account</strong>
          <div>This area is for {role} users. You are signed in as {user.role}.</div>
        </div>
      </Page>
    )
  }
  return <>{children}</>
}

function NotFound() {
  return (
    <Page narrow>
      <div className="empty">
        <h3>Page not found</h3>
        <div className="small">The link may be out of date.</div>
      </div>
    </Page>
  )
}
