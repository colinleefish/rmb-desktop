# RMB Desktop

[English](README.md) / 简体中文

**让你的 AI 编程助手拥有长期记忆。**

RMB 会记住你在 Cursor、Claude Code、Codex 等工具里说过的话，这样它们就不会每个会话都重新问你同样的问题。全部在你自己的电脑上运行，数据留在本地。

官网：[re-mem-ber.me](https://re-mem-ber.me)

## 它能做什么

AI 助手一关掉对话就会忘掉一切。RMB 用三步解决这个问题：

1. **捕获** — 静默记录你与编程助手的对话
2. **记忆** — 在后台把对话提炼成有用的事实
3. **召回** — 让助手在再次问你之前，先搜索已有知识

可以把它当成所有 AI 助手共用的一本笔记本。

## 适合谁用

经常用 AI 编程工具，却厌倦一遍遍重复项目背景、个人偏好或已做过决定的人。

支持：

- [x] Cursor
- [x] Claude Code
- [x] Codex
- [x] OpenCode
- [x] Pi

## 下载与安装

1. 从 [re-mem-ber.me](https://re-mem-ber.me) 或 [GitHub Releases](https://github.com/colinleefish/rmb-desktop/releases) 下载
2. 打开应用 — 菜单栏会出现 RMB 图标
3. 按引导完成设置（选择助手 + 填入 API Key）
4. 继续写代码 — RMB 在后台自动工作


### 如果提示「已损坏，无法打开」

当前版本未做 Apple 公证，浏览器下载带隔离属性时会被 Gatekeeper 按「已损坏」拦截。把应用拖进 `Applications` 后，在终端执行以下命令再重新打开即可：

```bash
xattr -dr com.apple.quarantine "/Applications/RMB Desktop.app"
```

## 隐私

- 完全在本地运行 — 不需要注册云账号
- 只有「把对话提炼成记忆」时才需要 LLM API Key
- 多设备同步以后可能会有；当前版本是单机使用

## 许可证

待定
