import { Link } from 'react-router-dom'
import { Ambient } from '../components/Ambient'
import { Icon } from '../components/Icon'

export function NotFound() {
  return (
    <div className="lg-login">
      <Ambient />
      <div className="login-stage" style={{ gridTemplateColumns: '1fr', maxWidth: 680 }}>
        <section className="login-hero glass" style={{ minHeight: 420, alignItems: 'flex-start' }}>
          <div className="brand">
            <span className="logo">X</span>
            <span>
              <b>x-admin</b>
              <span>OPERATIONS WORKBENCH</span>
            </span>
          </div>
          <h1 style={{ fontSize: 64, marginTop: 40 }}>404</h1>
          <p className="lede">这个页面不在工作台上。可能已移动、删除，或你没有访问权限。</p>
          <Link className="btn primary" to="/dashboard" style={{ marginTop: 26, textDecoration: 'none' }}>
            <Icon name="arrowRight" size={13} /> 返回总览
          </Link>
        </section>
      </div>
    </div>
  )
}
