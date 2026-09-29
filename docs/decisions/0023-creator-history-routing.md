# ADR-0023：Creator普通路径路由

- **状态**：Accepted
- **日期**：2026-09-20
- **决策者**：BaSui（明确要求改成普通路径）
- **关联任务**：INK-215
- **替代关系**：替代Creator现有Hash导航实现；不改变Admin路由与API契约

## 背景（Context）

用户要求首页及独立登录页采用普通路径。此前Creator使用Hash导航，静态托管没有明确的深链刷新配置。Creator与Admin仍为独立应用。

## 决策（Decision）

Creator使用浏览器History API，首页为/，其余现有视图为/login、/register、/forgot-password、/outline、/worldview、/analysis、/editor。统一路由模块负责解析、pushState/replaceState与popstate订阅，不新增路由依赖。

returnTo只接受Creator同源已知非鉴权页面；编辑器作品ID放入URL查询参数以支持刷新与登录回跳。认证守卫和成功回跳使用replace，避免后退反复进入守卫页。旧Hash链接首次进入转换为普通路径，不保留第二套路由。

Vite显式使用SPA模式；生产静态托管需对已知页面回退index.html，API和静态资源保持独立处理。提供可部署Nginx配置示例，实际部署仍由部署环境选择和装配。

## 备选方案（Alternatives）

Hash部署简单但不满足用户要求。引入第三方路由库有更完整嵌套路由能力，但当前少量平级页面无需额外依赖。服务端逐页渲染改变应用运行方式，超出本次范围。

## 后果（Consequences）

地址可读、支持深链；部署必须提供HTML回退且不得覆盖API。浏览器路由白名单与静态托管页面清单需同步。后端HTTP、身份保护和七域调用链不变。

## 验证方式（Verification）

Go与双端构建；浏览器首页登录跳转、刷新、注册保留来源、前进后退；路由函数内存断言外跳拒绝和作品参数保留；构建预览HTTP请求/login与/outline返回HTML、/v1不存在接口不返回HTML。生产Nginx示例独立校验，未部署部分如实标明。
