import { NavLink } from 'react-router-dom'

export function AccountTabs() {
  return (
    <div className="tabs">
      <NavLink to="/account" end>Rentals</NavLink>
      <NavLink to="/account/invoices">Invoices</NavLink>
      <NavLink to="/account/profile">Details</NavLink>
    </div>
  )
}
