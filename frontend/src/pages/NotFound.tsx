/**
 * 404 页面：在后台框架内展示“路由未命中”，并提供回到工作台的路径。
 */
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Icon } from '../components/Icon'
import { profileNav, useNav } from '../config/nav'

export function NotFound() {
  const location = useLocation()
  const navigate = useNavigate()
  const { items } = useNav()

  // 首页与快捷入口都取自当前账号实际可访问的菜单，避免把无权限页面当成出口。
  const home = items[0] ?? profileNav
  const quickLinks = items.filter((item) => item.path !== home.path).slice(0, 3)

  return (
    <div className="notfound-page">
      <section className="notfound-panel glass">
        <div className="notfound-art" aria-hidden="true">
          <svg className="notfound-map" viewBox="0 0 360 320">
            <circle className="notfound-ring" cx="180" cy="160" r="124" />
            <circle className="notfound-ring soft" cx="180" cy="160" r="86" />
            <path
              className="notfound-route"
              d="M36 236h78c28 0 42-18 58-42l24-34c8-11 18-17 31-17h35"
            />
            <path className="notfound-route dashed" d="M262 143h28c10 0 18-4 26-12" />
            <circle className="notfound-node" cx="36" cy="236" r="4" />
            <circle className="notfound-node lost" cx="262" cy="143" r="5" />
            <path className="notfound-cross" d="m251 132 22 22m0-22-22 22" />
          </svg>
          <span className="notfound-code">404</span>
          <span className="notfound-caption">页面路由未命中</span>
        </div>

        <div className="notfound-copy">
          <span className="notfound-badge">
            <i />
            404 · 页面不存在
          </span>
          <h1>这条路径没有对应的工作台页面</h1>
          <p>
            地址可能拼写错误，或页面已经移动、删除；也可能是当前账号没有访问权限。
            你可以返回上一页，或从下面的入口继续。
          </p>

          <div className="notfound-path">
            <span>尝试访问</span>
            <code title={location.pathname}>{location.pathname}</code>
          </div>

          <div className="notfound-actions">
            <Link className="btn primary" to={home.path}>
              <Icon name="arrowRight" size={13} /> 回到{home.label}
            </Link>
            <button className="btn" type="button" onClick={() => navigate(-1)}>
              <Icon name="chevronLeft" size={13} /> 返回上一页
            </button>
          </div>

          {quickLinks.length > 0 && (
            <div className="notfound-links">
              <span>快速前往</span>
              {quickLinks.map((item) => (
                <Link key={item.path} to={item.path}>
                  {item.label}
                </Link>
              ))}
            </div>
          )}
        </div>
      </section>
    </div>
  )
}
