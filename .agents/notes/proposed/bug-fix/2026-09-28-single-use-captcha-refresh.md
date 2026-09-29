# Agent Note: 单次验证码失败后刷新挑战

Status: proposed

Task: INK-238
Scope: frontend/packages/ui/src/components/AuthView.tsx 与 GraphicCaptcha 登录挑战状态

## Problem

服务端 `Captcha.Verify` 每次校验后消费 challenge。若验证码正确但账号密码错误，登录返回401时验证码也已失效；共享 AuthView 却保留旧图片与答案，用户重试会得到400。手动换图时输入框也可能残留旧答案。

## Proposal

登录请求失败时保留原错误提示、清空验证码文本与ID并重新挂载 `GraphicCaptcha` 获取新挑战。验证码 change 回调同步清除旧答案并更新本地回退码。后端的一次性校验与防重放语义保持不变。

## Alternatives considered

- 后端允许重复使用验证码：扩展了挑战重放窗口，违反当前一次性消费约束。
- 仅显示“请手动刷新”：用户容易继续提交已消费的 ID；失败后主动获取新挑战更稳定。

## Acceptance criteria

- 登录失败后图片与 captcha ID 换新，输入内容清空，错误提示仍保留。
- 用户手动换图后清除旧答案；新图只搭配新 ID 提交。
- Admin 用户名root和邮箱登录、Creator 登录原有验证码载荷映射不变。

## Risks

验证码状态仍由单个后端进程保存，多实例需要共享状态存储。当前只完成构建与接口就绪检查；因端到端校验需要读取并提交图片验证码，交互验证等待现场确认。
