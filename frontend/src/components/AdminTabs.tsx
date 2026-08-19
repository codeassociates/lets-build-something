import { NavLink } from 'react-router-dom'
import { useAuth } from '../auth'

export function AdminTabs() {
  const { can } = useAuth()
  return (
    <div className="tabs">
      <NavLink to="/admin" end>Overview</NavLink>
      <NavLink to="/admin/items">Catalog</NavLink>
      <NavLink to="/admin/fleet">Fleet</NavLink>
      {can('admin') && <NavLink to="/admin/users">People</NavLink>}
      <NavLink to="/admin/emails">Email log</NavLink>
      {can('admin') && <NavLink to="/admin/jobs">Jobs</NavLink>}
    </div>
  )
}
