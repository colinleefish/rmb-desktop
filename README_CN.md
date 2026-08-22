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


### 首次启动（macOS）

发布版本已签名并公证（Developer ID），首次打开只需按常规确认即可。如果安装的是早期未签名版本并被提示「已损坏」，在终端执行以下命令后重新打开：

```bash
xattr -dr com.apple.quarantine "/Applications/RMB Desktop.app"
```

## 隐私

- 完全在本地运行 — 不需要注册云账号
- 只有「把对话提炼成记忆」时才需要 LLM API Key
- 多设备同步以后可能会有；当前版本是单机使用
- **本地搜索遥测（使用热度）**：rmbd 会在本地 SQLite 中记录每次搜索的
  查询词、scope、k、前 k 条结果 URI，以及 10 分钟内是否有人 `cat` 其中
  一条（即「这次搜索是否真正回答了问题」），并维护每个 URI 的使用热度
  计数。这些数据仅用于检索健康度指标与排序校准，**永不上传、不导出、
  不联网**；卸载时随数据库一起删除。纯搜索曝光不计入热度 — 只有显式
  读取（`cat`/`meta`）才计。

## 许可证

待定
