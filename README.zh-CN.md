# vault-manager

[English](README.md) · [Русский](README.ru.md) · [Español](README.es.md) ·
[简体中文](README.zh-CN.md)

![vault-manager 界面预览](docs/images/vault-manager.svg)

`vault-manager` 是一个独立的 HashiCorp Vault 集群交互式终端管理器。它只与
Vault HTTP API 通信，不依赖外部身份源、配置系统或特定部署环境的同步机制。

当前版本：`0.2`。

预构建归档及其校验和可从
[GitHub Releases 页面](https://github.com/shurrman/vault-manager/releases/latest)下载。

界面由三个面板组成：

1. Groups、Users、Engines 等功能分区；
2. 当前分区中的项目；
3. 当前项目的属性与操作。

界面优先显示易于理解的名称。Vault ID 和 accessor 仍会在有助于诊断时显示在
预览中。

## 当前功能

版本 0.2 可管理 Status、Groups、Users、Policies、Engines、SSH、
Auth Methods、Audit 和 Tokens：

| 区域 | 可用操作 |
| --- | --- |
| 集群 | Seal、提交 unseal 密钥、保存和恢复 Raft 快照 |
| 内部组 | 创建、编辑策略/成员关系/元数据、删除 |
| 用户 | 创建 userpass、编辑 entity 和密码、撤销 userpass、删除 entity |
| ACL 策略 | 创建、编辑多行 HCL/JSON 正文、删除 |
| Secrets engines | 浏览并读取现有值层级、启用、禁用 |
| SSH 角色 | 创建、编辑完整 JSON 配置、删除 |
| 认证方法 | 调整、启用、禁用 |
| 审计设备 | 启用、禁用 |
| Tokens | 按 accessor 撤销 |

预览中的多行值会保留换行和缩进。每个编辑器底部都有 Help 区域，提供语法、
示例和操作提示。破坏性操作会显示确认界面，并默认选择 Cancel；Vault 仍会对
每项操作检查调用 token 的 ACL 权限。

逗号分隔的列表字段会从当前 Vault 对象中显示筛选后的候选项，排除已选值，
并支持使用上/下方向键和 `Enter`/`Tab` 选择。HCL/JSON 字段会在确认前和 API
写入前再次验证，语法错误会显示行号和列号。

TUI 语言由 `ui.language` 设置，可选 `en`、`ru`、`es` 或 `zh`；未设置时
默认为 `en`。

## 构建

开发需要 Go 1.25 或更高版本。默认构建会生成使用基础 x86-64 指令集的静态
Linux amd64 二进制文件：

```sh
make build
```

构建结果位于 `bin/vault-manager`。构建使用 `CGO_ENABLED=0` 和
`GOAMD64=v1`，以兼容从 CentOS 7 到 Debian 13 的系统。

使用以下命令生成完整的发布归档矩阵：

```sh
make release
```

| 操作系统 | 架构 | 归档 |
| --- | --- | --- |
| Linux | amd64 | `vault-manager_0.2_linux_amd64.tar.gz` |
| Linux | arm64 | `vault-manager_0.2_linux_arm64.tar.gz` |
| macOS | amd64 | `vault-manager_0.2_darwin_amd64.tar.gz` |
| macOS | arm64 | `vault-manager_0.2_darwin_arm64.tar.gz` |
| Windows | amd64 | `vault-manager_0.2_windows_amd64.zip` |

所有二进制文件均使用 `CGO_ENABLED=0` 构建；amd64 构建使用
`GOAMD64=v1`，arm64 构建使用基础级别 `GOARM64=v8.0`。
`dist/checksums.txt` 包含五个归档的 SHA-256 校验和。推送两段式 `vX.Y`
标签后，GitHub Actions 会运行测试、构建此矩阵并自动创建或更新 GitHub Release。

## 配置

将 `.vault-manager.example` 复制为 `$HOME/.vault-manager`，替换 Vault 地址和
token，并将文件权限限制为仅所有者可访问：

```sh
cp .vault-manager.example "$HOME/.vault-manager"
chmod 600 "$HOME/.vault-manager"
```

```toml
[vault]
address = "http://vault.wsoft:8400"
token = "replace-me"
request_timeout = "10s"

[ui]
color = "auto"
language = "zh"
```

`ui.language` 接受双字母代码 `en`、`ru`、`es` 和 `zh`，默认值为 `en`。
在可信内部反向代理网络中支持不使用 TLS 的 HTTP，例如
`http://vault.wsoft:8400`。

未指定 `--config` 时，程序始终加载 `$HOME/.vault-manager`，不受当前工作目录
影响。如果组用户或其他用户可以读取配置文件，程序将拒绝使用该文件。
`.vault-manager` 和 `.env` 均已被 Git 忽略。

如果某个部署环境从其他系统同步组成员关系，可以在不修改应用程序的情况下添加
本地提示：

```toml
[deployment]
membership_notice = "Membership is managed externally and may change after synchronization."
```

这段文字会在编辑用户组之前显示。它仅用于提示；程序核心不会调用或解析外部系统。

## 运行

```sh
bin/vault-manager
```

使用 `bin/vault-manager --version` 显示应用版本。使用
`--config /path/to/config.toml` 指定其他配置文件。

只读连接检查命令：

```sh
bin/vault-manager --check
```

该检查会列出每个分区，并加载其中第一个项目的预览，不执行任何写操作。

## 键盘操作

- `↑`/`↓`、`Home`、`End`：在当前面板中移动；
- `←`/`→`：在面板之间移动；
- `Tab`/`Shift+Tab`：切换到右侧或左侧面板；在中间面板按 `Tab` 会选中
  预览中的第一个操作；
- `Enter`：激活项目、操作或编辑器字段；
- 在逗号分隔的列表字段中，`↑`/`↓` 选择筛选后的候选项，`Enter` 或
  `Tab` 插入候选项；
- 在 Engines 的 `Path` 行按 `Enter` 会直接打开值浏览器；在浏览器内，`Enter`
  进入/打开，`Esc` 或 `←` 返回上一级路径；
- `Space`：切换编辑器中的布尔字段；
- `Ctrl+S`：检查当前编辑器操作；
- 在多行字段中按 `Enter`：插入新行；
- `Esc`：接受多行字段，或检查/退出编辑器；
- `r`：刷新当前数据；
- `?`：显示简短的按键提示；
- `q` 或 `Ctrl+C`：未编辑文本字段时退出。

终端支持时会使用颜色。设置 `ui.color = "never"`、存在 `NO_COLOR` 环境变量
或终端不支持颜色时，`*` 表示选中的路径，`>` 表示当前活动项目。终端最小
尺寸为 80x20。

## 成员关系与保存安全

Vault 将 entity 的直接成员关系存储在内部组对象中。因此，修改用户的 Groups
字段可能需要多次顺序写入，这一过程不是原子操作。写入前，`vault-manager`
会检查 entity 和所有受影响的组是否发生并发更改，然后按组名的确定顺序更新各组，
并验证每次更新的结果。

如果后续写入失败，之前成功的写入不会回滚。界面会列出已应用、失败和未尝试的
操作，重新加载 Vault 的实际状态，并要求重新打开编辑器后再继续。

其他编辑器同样会将对象的当前状态与最初加载的版本进行比较，并拒绝覆盖并发更改。
密码和 token 不会完整显示、写入日志或包含在错误消息中。

## 开发检查

```sh
make test
make test-race
make vet
```

API 写入测试使用进程内的模拟 Vault HTTP 服务器。配置的集群只会由显式的只读
`--check` 命令访问。

设计详情请参阅 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)（英文）。
