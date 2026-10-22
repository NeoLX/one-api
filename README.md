<p align="right">
   <strong>中文</strong> | <a href="./README.en.md">English</a>
</p>

<div align="center">

# One API · NeoLX 增强版

_✨ 基于 [songquanpeng/one-api](https://github.com/songquanpeng/one-api) 的私有分支：多渠道大模型网关 + 渠道自定义请求头 / 上游模型拉取 / OpenCode Go 订阅渠道 / 用户级代理白名单 ✨_

</div>

> [!IMPORTANT]
> 本仓库是 [One API](https://github.com/songquanpeng/one-api) 的 fork（`NeoLX/one-api`），在上游基础上叠加了若干实用增强，见下文「本分支的新增特性」。
> 上游的部署方式、环境变量、管理 API 均适用于本分支；与上游的差异与用法详解见文末「与上游的差异」。
>
> 使用须遵循 OpenAI [使用条款](https://openai.com/policies/terms-of-use)与相关法律法规；请勿对公众提供未经备案的生成式人工智能服务。

---

## 本分支的新增特性

### 1. 渠道自定义请求头（Headers）

渠道配置新增 `Headers` 字段：在渠道编辑页以 JSON 填写任意请求头，**中继请求与余额查询都会自动套用**。

- 内置黑名单，`Host`、`Content-Length` 等会破坏请求的头会被挡掉，不必担心误填；
- 适合需要特殊认证头、`Referer`、UA 校验或走网关鉴权的下游供应商。

```json
{ "X-Api-Key": "xxx", "User-Agent": "one-api/1.0" }
```

### 2. 渠道一键从上游拉取模型列表

渠道页新增「拉取模型」按钮：按渠道类型自动推导模型列表端点与认证方式，一键获取上游真实支持的模型，不再手抄。

- 支持 OpenAI 及所有 OpenAI 兼容格式渠道（含自定义 Base URL）；
- 特殊渠道单独适配：OpenCode Go 的列表端点在 `/v1` 上一层且无需认证；
- 拉取结果可在编辑页勾选后再保存进渠道。

### 3. OpenCode Go 订阅套餐渠道（类型 #52）

内置 [OpenCode Go](https://opencode.ai) 订阅套餐渠道：

- 聊天请求走 `/inference/go/openai/v1`；
- 模型列表位于上一层 `/inference/go/v1/models`，无需认证即可拉取。

### 4. 用户级「代理」白名单

普通用户可在「代理」页面**自助配置「我能用哪些供应商的哪些模型」**：

- 可选项来自该用户分组内本来就能用的渠道，**只做减法**，不能越权；
- 键为渠道类型（供应商），值为允许的模型列表；某供应商配置为空列表表示放行其全部模型；
- **未配置等于不限制**，非法配置自动降级为不限制并记录系统日志，不会拦人；
- 中继分发渠道时按白名单过滤，与令牌模型限制、分组限制叠加生效。

### 5. 令牌密钥脱敏显示

令牌列表中的 `sk-` 密钥默认脱敏显示（`sk-••••••••abcd`），点击可复制到完整密钥，避免肩窥泄露。

### 6. 访问信息页

新增「访问信息」页：一屏看清自己的 **Base URL、可用模型清单**，以及各端点
（`/v1/chat/completions`、`/v1/completions`、`/v1/embeddings`、`/v1/audio/speech`、`/v1/audio/transcriptions`、`/v1/images/generations`）的调用示例，一键复制，方便直接填进各类客户端。

### 7. 国内镜像一键构建（Dockerfile.cn）

新增 `Dockerfile.cn` 与 `.dockerignore`，apk、Go module 全部走国内镜像源（阿里云 / goproxy.cn），适合国内网络环境本地构建：

```bash
# 1) 宿主机构建前端
cd web/default
npm install --registry=https://registry.npmmirror.com
DISABLE_ESLINT_PLUGIN=true CI=false ./node_modules/.bin/react-scripts build
rm -rf ../build/default && mv build ../build/default

# 2) 构建镜像（只编译 Go 后端）
cd ../..
docker build --progress=plain -f Dockerfile.cn -t one-api:local .

# 3) 运行
docker run --name one-api -d --restart always -p 3000:3000 \
  -e TZ=Asia/Shanghai -v ./data:/data one-api:local
```

---

## 继承自上游的核心能力

1. 多模型统一接入：以标准 OpenAI API 格式中继 ChatGPT / Claude / Gemini / 豆包 / 文心 / 通义 / 星火 / GLM / DeepSeek 等 30+ 供应商（清单见[上游 README](https://github.com/songquanpeng/one-api#功能)）。
2. 多渠道**负载均衡**与失败自动重试，支持 stream 流式输出。
3. 令牌管理：过期时间、额度、IP 范围、允许模型；令牌后拼渠道 ID 可锁定渠道（`sk-xxx-渠道ID`）。
4. 用户/渠道分组与倍率、兑换码、额度明细、邀请奖励、公告与充值配置。
5. 多渠道批量创建、模型映射、绘图接口、Cloudflare AI Gateway 对接。
6. 多机部署（主从 + MySQL + Redis），管理 API 可在不二开的前提下扩展（[API 文档](./docs/API.md)）。
7. 多种登录方式（邮箱 / GitHub / 飞书 / 微信公众号）、Cloudflare Turnstile 校验。

## 快速开始（与上游一致）

```bash
docker run --name one-api -d --restart always -p 3000:3000 \
  -e TZ=Asia/Shanghai -v /home/ubuntu/data/one-api:/data \
  justsong/one-api
```

> 注意：官方镜像不含本分支增强；要用新增特性请使用本仓库镜像或按上文 `Dockerfile.cn` 自行构建。

初始账号 `root`，密码 `123456`，登录后**务必修改密码**。

使用方式与 [OpenAI API](https://platform.openai.com/docs/api-reference/introduction) 一致，把 API Base 指向部署地址（如 `https://your-host/v1`），API Key 填 One API 令牌即可。

## 与上游的差异

| 方面 | 说明 |
|---|---|
| 新增后端 | `controller/channel-models.go`（拉取模型）、`controller/user_proxy.go` + `model/user_proxy.go`（用户代理白名单）、`relay/adaptor/header.go`（渠道 Headers）、渠道类型 #52 OpenCode Go |
| 新增前端 | 渠道编辑页 Headers/拉取模型、用户「代理」页、令牌列表密钥脱敏、「访问信息」页 |
| 构建 | 新增 `Dockerfile.cn` / `.dockerignore`（国内镜像源）；`.gitignore` 忽略本地开发笔记目录 |
| 数据 | `users` 表新增 `proxy_config` 列（首次启动自动迁移）；渠道 Headers 存于既有 `config` JSON 字段，无需加列 |

环境变量、多机部署、常见问题等其余文档请直接参考[上游 README](https://github.com/songquanpeng/one-api#部署)。

---

## 许可与出处

本项目基于 [songquanpeng/one-api](https://github.com/songquanpeng/one-api)（MIT 协议）进行二次开发。

依据上游 MIT 协议**附加条款**，页面底部必须保留上游署名及链接：

> 本系统基于 [One API](https://github.com/songquanpeng/one-api) 构建，感谢原作者 [songquanpeng](https://github.com/songquanpeng)。

依据 MIT 协议，使用者需自行承担使用本项目的风险与责任。
