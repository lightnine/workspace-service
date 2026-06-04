# WeData 3.0 Studio 架构演进：Workspace Server + 共享 JuiceFS

> 范围：本文聚焦 Studio 模块的架构重构想法，核心是用 Go 实现统一的 **Workspace Server**，收敛当前 OneFlow 控制面与 `wedata-jupyter-server` 的职责；底层存储从 CFS 切换到 **单一共享 JuiceFS**，靠挂载路径做租户与 workspace 隔离。
>
> 关联文档：`wedata3_studio_go_unified_technical_proposal.md`（采用 Per-Tenant JuiceFS 方案，与本文存储隔离策略不同，详见第 5 节对比）。

## 1. 当前架构

### 1.1 承担的功能

用户能够在页面创建 Notebook、编写代码、连接 DLC 引擎 Kernel、执行代码。

### 1.2 运行形态

每个用户启动一个个人 Pod，Pod 内运行三个容器：

| 容器 | 职责 |
|---|---|
| `jupyter-server` | 文件服务、Notebook Session / Kernel 管理、连接 DLC EG、WebSocket 代理 |
| `code-server` | Web IDE 编辑能力 |
| `fuse-proxy` | 通过 FUSE 代理访问底层存储 |

底层存储采用 **CFS**。

### 1.3 文件操作链路

前端操作文件时，接口请求到 **OneFlow** 服务，OneFlow 操作本地业务数据库，再请求基础平台完成文件的创建。

### 1.4 当前架构图

```mermaid
flowchart LR
    subgraph fe [Frontend]
        studioFe["Studio Frontend"]
    end

    subgraph oneflow [OneFlow Service]
        ofApi["OneFlow API"]
        bizDb[("Business DB")]
    end

    subgraph platform [Platform]
        platformBiz["Base Platform"]
    end

    subgraph userPod [Per-User Pod]
        jupyter["jupyter-server"]
        codeServer["code-server"]
        fuse["fuse-proxy"]
    end

    subgraph dlc [DLC Runtime]
        eg["DLC Enterprise Gateway"]
        kernel["DLC Kernel / Spark App"]
    end

    cfs[("CFS")]

    studioFe --> ofApi
    studioFe -->|"edit / file / kernel"| jupyter
    ofApi --> bizDb
    ofApi -->|"create file"| platformBiz
    jupyter --> eg
    eg --> kernel
    jupyter --> fuse
    fuse --> cfs
    kernel -->|"mount"| cfs
```

### 1.5 现有架构的痛点

| 痛点 | 说明 |
|---|---|
| 个人 Pod 启动开销 | 进入 Studio 需等待个人 Pod 创建/唤醒，文件浏览与编辑被 Pod 生命周期绑定 |
| 职责分散 | 文件创建在 OneFlow + 基础平台，Kernel/Session 在 jupyter-server，链路长、状态分散 |
| 容器冗余 | `code-server`、`fuse-proxy` 增加 Pod 复杂度和资源占用 |
| CFS 网络约束 | CFS 适合同 VPC / 对等连接挂载，跨网络部署受限，且依赖 fuse-proxy 代理 |

## 2. 新架构目标

两条主线：

1. **统一控制面**：用一个 Go 编写的 **Workspace Server** 收敛 OneFlow 与 jupyter-server 的核心职责，去掉 `code-server` 和 `fuse-proxy`，不再为每个用户启动个人 Pod 来承载文件与 Kernel 代理。
2. **共享存储 + 路径隔离**：底层存储改为 **单一共享 JuiceFS**（依赖 MySQL + COS）。Workspace Server 与 DLC 引擎 Pod 都挂载同一个 JuiceFS，不同租户、不同 workspace 之间通过 **JuiceFS 挂载路径** 进行数据隔离。

### 2.1 Workspace Server 职责

| 能力 | 说明 |
|---|---|
| 文件管理 | 创建、删除、重命名、移动、列表、文件树、内容读写（直接操作 JuiceFS） |
| Kernel 管理 | 创建/重启/停止 Kernel，与 DLC EG 交互，生成启动参数，代理 Jupyter WebSocket |
| Session 管理 | 维护 Notebook 与 Kernel 的绑定关系、会话状态对账 |
| 收敛 OneFlow | 文件创建不再绕基础平台，由 Workspace Server 直接落 JuiceFS + 业务库 |

## 3. 新版架构图

```mermaid
flowchart TB
    %% ---------- Layer 1: Frontend ----------
    subgraph fe [" 前端 Frontend "]
        direction LR
        studioFe["Studio Frontend"]
        notebookUi["Notebook UI"]
    end

    %% ---------- Layer 2: Access ----------
    gateway["API Gateway / Ingress"]

    %% ---------- Layer 3: Workspace Server ----------
    subgraph ws [" Workspace Server (Go) "]
        direction TB
        api["API Adapter<br/>TRPC / REST / Jupyter HTTP / WebSocket"]
        subgraph wsmod [" "]
            direction LR
            fileSvc["File Service"]
            sessionSvc["Session / Kernel Service"]
            envSvc["Mount Builder"]
            wsProxy["WebSocket Proxy"]
        end
        api --> fileSvc
        api --> sessionSvc
        api --> wsProxy
        sessionSvc --> envSvc
    end

    %% ---------- Layer 4: backing services ----------
    subgraph state [" 状态 State "]
        direction LR
        bizDb[("Business DB<br/>MySQL")]
        redis[("Redis")]
    end

    subgraph dlc [" DLC Runtime "]
        direction TB
        dlcApi["DLC Cloud API"]
        eg["DLC Enterprise Gateway"]
        kernel["DLC Kernel Pod /<br/>Spark App"]
        dlcApi --> eg
        eg --> kernel
    end

    subgraph storage [" 共享存储 Shared JuiceFS "]
        direction TB
        juicefs["JuiceFS<br/>(single instance, all tenants)"]
        jfsMeta[("Metadata<br/>MySQL")]
        cos[("Data Blocks<br/>COS")]
        juicefs --> jfsMeta
        juicefs --> cos
    end

    %% ---------- cross-layer edges ----------
    fe --> gateway
    gateway --> api

    fileSvc --> bizDb
    sessionSvc --> redis
    sessionSvc --> dlcApi
    wsProxy --> eg

    fileSvc -. "mount & R/W<br/>/{tenantId}/{workspaceId}/" .-> juicefs
    kernel -. "mount<br/>/{tenantId}/{workspaceId}/" .-> juicefs
```

> 说明：实线为同步调用 / 控制流，虚线为 JuiceFS 挂载与读写。Workspace Server 挂载共享 JuiceFS 根或 workspace 子路径；DLC Kernel Pod 挂载当前登录用户路径 `/{tenantId}/{workspaceId}/users/{userUin}/`。

### 3.1 核心交互关系

| 交互 | 说明 |
|---|---|
| 前端 → Workspace Server | 文件树、文件内容、Notebook 编辑、Kernel 创建、WebSocket 执行统一进入同一个 Workspace Server |
| Workspace Server → JuiceFS | Workspace Server 挂载共享 JuiceFS，按 `tenantId/workspaceId` 路径读写文件 |
| Workspace Server → DLC | 查询引擎信息、生成 Kernel 参数、创建 Kernel、代理 WebSocket |
| DLC Kernel → JuiceFS | Kernel Pod 挂载同一个共享 JuiceFS，挂载到当前登录用户路径，读写 Notebook、依赖与运行产物 |
| JuiceFS → MySQL + COS | JuiceFS 元数据存 MySQL，数据块存 COS |

### 3.2 与当前架构的差异

| 维度 | 当前架构 | 新架构 |
|---|---|---|
| 控制面 | OneFlow + jupyter-server 分散 | 统一 Workspace Server（Go） |
| 用户运行环境 | 每用户个人 Pod（jupyter-server + code-server + fuse-proxy） | 无个人 Pod，去掉 code-server / fuse-proxy |
| 文件创建链路 | 前端 → OneFlow → 基础平台 | 前端 → Workspace Server → JuiceFS + 业务库 |
| 底层存储 | CFS（经 fuse-proxy） | 共享 JuiceFS（MySQL + COS） |
| 存储挂载 | fuse-proxy 代理 | Workspace Server 与 Kernel Pod 直接挂载 JuiceFS |

## 4. 存储设计：单一共享 JuiceFS + 路径隔离

### 4.1 组成

| 组件 | 作用 |
|---|---|
| JuiceFS | 提供 POSIX 文件系统语义，统一被 Workspace Server 和 Kernel Pod 挂载 |
| MySQL | JuiceFS 元数据引擎（目录树、inode、文件属性） |
| COS | JuiceFS 数据块对象存储 |

所有租户共用**同一个** JuiceFS 文件系统，不为每个租户单独部署 JuiceFS 实例。

### 4.2 路径隔离模型

不同租户、不同 workspace 通过挂载路径隔离：

```text
juicefs-root/
  {tenantId}/
    {workspaceId}/
      users/
        {userUin}/
          {repoName}/        # 用户可直接在个人目录下 git clone 仓库
      shared/
      env/
        {envId}/{envVersionId}/
      trash/
```

| 隔离层级 | 隔离方式 |
|---|---|
| 租户隔离 | 路径前缀 `/{tenantId}/` |
| Workspace 隔离 | 路径前缀 `/{tenantId}/{workspaceId}/` |
| 用户隔离 | `/{tenantId}/{workspaceId}/users/{userUin}/` |

> Git 仓库不设置独立的顶层 `git/` 目录。用户在个人目录 `/{tenantId}/{workspaceId}/users/{userUin}/` 下直接执行 `git clone`，仓库目录表现为普通用户文件夹，例如 `users/{userUin}/{repoName}/`。前端 Git API 与 Terminal Git 都应以该用户目录为合法工作区边界。

### 4.3 挂载策略

| 端 | 挂载方式 |
|---|---|
| Workspace Server | 挂载 JuiceFS 根（或按需 subdir），按请求上下文解析到 `/{tenantId}/{workspaceId}` 子路径读写 |
| DLC Kernel Pod | 通过 CSI Driver 挂载 JuiceFS，挂载点限定到当前登录用户路径 `/{tenantId}/{workspaceId}/users/{userUin}/`，Kernel 进程不可见其他用户路径 |

### 4.4 隔离边界与风险

由于所有租户共用一个 JuiceFS 实例，隔离依赖路径与挂载控制，需要补强：

| 风险 | 应对 |
|---|---|
| 路径逃逸 | 所有文件操作先经 Workspace Server 解析到合法子路径，禁止直接信任前端传入路径；Kernel 侧挂载点限定 subdir |
| 凭证爆炸半径 | 单一 JuiceFS 凭证泄露影响全部租户，需对挂载凭证强管控、最小权限、定期轮转 |
| 元数据库压力 | 所有租户共用一套 MySQL 元数据，需评估 inode 规模、热点、限流与扩容方案 |
| 噪声邻居 | 大租户读写可能影响他人，需配额、限速与监控 |
| 误删/越权 | 业务层 ACL 必须在 Workspace Server 控制面完成，不依赖文件系统权限 |

### 4.5 与 Per-Tenant JuiceFS 方案对比

| 维度 | 本文：共享 JuiceFS + 路径隔离 | 另一方案：Per-Tenant JuiceFS |
|---|---|---|
| 实例数量 | 单一实例，所有租户共用 | 每租户独立元数据库与凭证 |
| 隔离强度 | 逻辑隔离（路径 + 挂载控制 + 业务 ACL） | 物理隔离（独立元数据库/凭证） |
| 凭证爆炸半径 | 全租户 | 单租户 |
| 运维复杂度 | 低（一套实例） | 高（多套生命周期管理） |
| 元数据扩展性 | 单库压力集中，需重点评估 | 压力按租户分散 |
| 适用阶段 | 快速落地、租户规模可控 | 强隔离合规、租户规模大 |

> 选型建议：首期可用共享 JuiceFS 快速打通，但需在第 4.4 节风险项上做好防护；若后续对隔离/合规要求升级，可平滑演进到 Per-Tenant JuiceFS。

### 4.6 挂载凭证安全（防止 MySQL / COS 凭证泄露）

本节要解决两个问题：

1. Workspace Server 自身要挂载共享 JuiceFS，用于文件树、文件读写、前端 Git、身份索引同步；
2. 用户连接 DLC 引擎时，DLC 会启动 driver pod / executor pod，这些 pod 也要挂载同一个 JuiceFS，但挂载路径必须和该登录用户路径一致，例如：

   ```text
   /{tenantId}/{workspaceId}/users/{userUin}/
   ```

用户可以同时启动多个 DLC 引擎，因此同一个登录用户可能同时存在多个 driver pod / executor pod。它们可以挂载同一个用户路径，但不能看到其他用户目录，更不能拿到 JuiceFS 的 MySQL / COS 凭证。

**核心风险**：DLC driver pod / executor pod 会运行用户任意代码，属于**不可信环境**。JuiceFS 社区版挂载需要：

- META URL（如 `mysql://user:pass@host/juicefs`）——含 MySQL 凭证；
- COS AK/SK——`juicefs format` 时写入元数据库，client 从 meta 读取。

若在 driver / executor 容器内执行 `juicefs mount`：

1. MySQL 凭证会出现在命令行参数 / 环境变量（`META_PASSWORD`），用户通过 `ps`、`/proc/self/environ`、history 即可读取；
2. 拿到 META URL 后可**直接连共享 MySQL 元数据库**，读/改所有租户元数据，甚至读出 format 时存入的 COS 凭证；
3. 拿到 COS AK/SK 后可绕过 Workspace Server / JuiceFS 挂载路径限制，直接访问对象存储中的数据块；
4. 用户启动多个 DLC 引擎时，如果每个业务容器都携带凭证，泄露面会随 pod 数量线性放大。

由于是**共享单 JuiceFS**，一旦泄露，爆炸半径是全部租户。

#### 4.6.1 信任边界

| 组件 | 是否可信 | 允许接触 JuiceFS 凭证 | 说明 |
|---|---|---|---|
| Workspace Server | 可信 | 可以 | 平台控制面，不运行用户代码，可通过 K8s Secret / Secret Manager 获取挂载凭证 |
| JuiceFS CSI Driver / node mount pod | 可信但高风险 | 可以 | 负责在 node 层执行挂载，必须受 RBAC、NetworkPolicy、节点权限保护 |
| DLC driver pod | 不可信 | 不可以 | 用户可进入 Terminal、执行 shell / Python / Git 命令，不能注入 MySQL/COS 连接串 |
| DLC executor pod | 不可信 | 不可以 | 运行用户作业代码，同样不能注入 MySQL/COS 连接串 |
| 前端 / Notebook / Terminal | 不可信 | 不可以 | 只能看到业务 API 或已挂载目录 |

设计原则：

- **凭证不进入用户代码容器**：driver / executor 容器的 env、启动参数、配置文件、挂载卷中都不能出现 MySQL URL、MySQL password、COS AK/SK；
- **挂载动作在可信边界完成**：Workspace Server 自身挂载由服务部署完成；DLC pod 的挂载由 CSI Driver / node mount pod 完成；
- **业务容器只看到挂载结果**：用户只能看到 `/workspace` 或约定的 mount path，看不到 `juicefs mount` 的启动参数和 Secret；
- **路径隔离由挂载参数和服务端校验共同保证**：DLC pod 只挂当前用户路径，Workspace Server 对所有 API path 再做一次 ACL 与前缀校验。

#### 4.6.2 推荐挂载模型

**Workspace Server 挂载：**

- Workspace Server 是平台后端服务，可以挂载 JuiceFS 根目录或 workspace 级子目录；
- JuiceFS 凭证只放在 Workspace Server 所在 namespace 的 K8s Secret / Secret Manager 中；
- Workspace Server 容器不对用户开放 shell，不运行用户提交的任意代码；
- Workspace Server 的 ServiceAccount 只能读取自身所需 Secret，不能读取 DLC namespace 中的其他 Secret；
- Workspace Server 对前端传入路径做规范化，禁止 `../`、软链接逃逸、跨 workspace / 跨用户路径访问。

**DLC driver / executor pod 挂载：**

- DLC pod 统一通过 JuiceFS CSI Driver 挂载，禁止在 driver / executor 容器内直接执行 `juicefs mount`；
- CSI 使用平台侧 Secret 完成挂载，Secret 只对 CSI Controller / Node Plugin 可见，不注入业务容器；
- Workspace Server / DLC 在创建引擎时生成挂载计划，将 subdir 限定为当前登录用户路径：

  ```text
  juicefsSubdir = /{tenantId}/{workspaceId}/users/{userUin}/
  containerMountPath = /workspace
  ```

- driver pod 和 executor pod 都只看到 `/workspace`，其根目录实际对应当前用户目录；
- 用户启动多个 DLC 引擎时，每个引擎的 driver / executor pod 都挂载同一个 `juicefsSubdir`。这会带来并发写入问题，但不会增加凭证暴露面，因为凭证仍只在 CSI 侧；
- 如果 executor 不需要访问 workspace 文件，优先不挂载；如果需要读取代码、依赖或输出文件，则挂载同一个用户路径。

推荐链路：

```text
Workspace Server
  -> 校验用户 / workspace / engine 权限
  -> 生成 MountPlan(tenantId, workspaceId, userUin, juicefsSubdir)
  -> 调用 DLC 创建 driver / executor pod
  -> DLC pod 通过 CSI 挂载 /{tenantId}/{workspaceId}/users/{userUin}/ 到 /workspace
  -> 用户代码只访问 /workspace，看不到 MySQL / COS 凭证
```

#### 4.6.3 凭证防泄露措施

| 措施 | 要求 |
|---|---|
| 禁止业务容器内 mount | driver / executor 镜像中不内置 JuiceFS 挂载脚本；即使有 `juicefs` 二进制，也没有 META URL / COS 凭证，无法完成挂载 |
| Secret 只给 CSI / Workspace Server | MySQL URL、MySQL password、COS AK/SK 只存在于平台 namespace 的 Secret / Secret Manager；DLC pod 的 env、ConfigMap、volumeMount 不引用这些 Secret |
| 不把连接串写入 Pod spec | DLC pod spec 里只能有 `tenantId/workspaceId/userUin/subdir` 这类非敏感挂载参数，不能出现 `mysql://user:pass@host`、AK/SK |
| RBAC 最小化 | DLC 用户 pod 的 ServiceAccount 不能 `get/list/watch secrets`，不能访问 CSI namespace，不能 `exec` 到 CSI / mount pod |
| NetworkPolicy 隔离 | driver / executor pod 不需要访问 JuiceFS MySQL 元数据库；如 COS 只由 node mount pod 访问，也应禁止业务 pod 直连 MySQL/COS 内网地址 |
| 进程与宿主机隔离 | 禁止 privileged、hostPID、hostNetwork、hostPath；避免用户 pod 通过宿主机 `/proc` 或 mount namespace 反查 CSI mount 进程参数 |
| Secret 加密与轮转 | K8s Secret 开启 etcd encryption；生产环境优先使用云 Secret Manager；MySQL/COS 凭证定期轮转 |
| 日志脱敏 | Workspace Server、DLC、CSI 组件日志不得打印 META URL、AK/SK、完整 mount command |
| 镜像与调试入口控制 | 不给用户 pod 授权调试 CSI / Workspace Server 容器；平台运维调试需要审计 |

其中最关键的是前三条：**业务容器内没有 Secret、Pod spec 里没有连接串、挂载由 CSI 代办**。只要这三条成立，用户在 Terminal 中通过 `env`、`ps aux`、`/proc/*/cmdline`、`/proc/*/environ` 都看不到 MySQL/COS 密码。

#### 4.6.4 路径隔离与多引擎并发

| 措施 | 角色 | 说明与局限 |
|---|---|---|
| 用户路径 subdir 挂载 | 必选 | DLC pod 的挂载根必须是 `/{tenantId}/{workspaceId}/users/{userUin}/`，而不是 workspace 根。这样用户代码无法通过相对路径进入其他用户目录 |
| Workspace Server 路径校验 | 必选 | 即使 Workspace Server 挂载根目录，对外 API 也必须按登录态校验 path 前缀，不允许用户传任意 JuiceFS 绝对路径 |
| 多 DLC 引擎同路径挂载 | 允许 | 同一用户启动多个 DLC 引擎时，多个 driver pod 可以挂载同一个用户路径。这不会泄露凭证，但可能同时修改同一 repo / 文件 |
| 并发写入控制 | 建议 | 对 Notebook 保存、前端 Git、Terminal Git 等高风险操作做 repo/path 级锁；否则只能接受最后写入覆盖或最后同步归属 |
| executor 挂载最小化 | 建议 | executor 只在确实需要读写 workspace 文件时挂载；训练中间结果优先写对象存储或临时目录，避免大量 executor 同时写用户代码目录 |

需要区分两个问题：

- **凭证安全**：由 CSI / Secret / RBAC / NetworkPolicy 解决。多个 DLC 引擎不会让用户拿到 MySQL/COS 连接串；
- **文件并发语义**：多个 DLC 引擎挂同一个用户目录，会产生同一文件被并发修改的问题。这是业务一致性问题，需要通过 repo/path 锁、保存冲突检测、Git 操作互斥、同步任务去重来解决，不应通过下发不同 JuiceFS 凭证来解决。

#### 4.6.5 仍需保留的纵深防御

| 措施 | 角色 | 说明与局限 |
|---|---|---|
| 网络 ACL | 推荐兜底（低成本） | 走 CSI 后 driver / executor pod 不需要到 MySQL 的网络通路，应直接切断；COS 如必须被用户作业访问，也应使用用户作业自己的数据凭证，不复用 JuiceFS COS 凭证 |
| MySQL 最小权限 | 必选 | JuiceFS client 对元数据库需要较高读写权限，无法做到租户级最小化；但必须避免 root 账号，禁止访问其他库，限制 DDL / 管理权限 |
| COS 权限最小化 | 必选 | JuiceFS 使用的 COS AK/SK 只允许访问该 JuiceFS bucket/prefix，不复用平台主账号高权限密钥 |
| COS 临时凭证（STS） | 可选、受限 | JuiceFS 的 COS 凭证供 JuiceFS client 使用，不下发给用户；CSI 下用户本就拿不到，STS 价值有限，且需确认 JuiceFS 对 token 刷新的支持能力 |
| 元数据访问代理 / 企业版 | 可选（强隔离） | 社区版 client 直连 MySQL 是结构性弱点；用 JuiceFS 企业版元数据服务或自建代理可避免把 MySQL 凭证放到每个 node mount 组件，适合更高隔离要求 |
| 凭证轮转 + 审计 | 运维基线 | 静态 Secret 仍建议定期轮转；CSI mount、Workspace Server 挂载、异常访问 MySQL/COS 都要留审计 |

首期推荐结论：

```text
Workspace Server：可信服务，可挂 JuiceFS 根或 workspace 根，凭证仅存在平台 Secret。
DLC driver/executor pod：不可信用户运行环境，只通过 CSI 获得 /workspace 挂载结果。
挂载路径：限定为 /{tenantId}/{workspaceId}/users/{userUin}/，多个 DLC 引擎可共享同一路径。
MySQL/COS 凭证：只给 Workspace Server 和 CSI/mount 可信组件，不进入用户 pod 的 env、命令行、配置文件或可读 volume。
```

## 5. Kernel 管理与 DLC EG 对接

Workspace Server 对前端保持 Jupyter 兼容协议，对下游连接 DLC EG，收敛原 jupyter-server 的 Kernel/Session 职责。

### 5.1 创建 Session / Kernel 流程

```mermaid
sequenceDiagram
    participant FE as Notebook UI
    participant WS as Workspace Server
    participant Env as Mount Builder
    participant EG as DLC EG
    participant K as Kernel Pod
    participant JFS as Shared JuiceFS
    participant DB as Business DB

    FE->>WS: POST /api/sessions
    WS->>WS: 校验 workspace / ACL
    WS->>Env: 生成 KernelLaunchPlan(env, KERNEL_MOUNT)
    Env-->>WS: env + 挂载路径 /{tenantId}/{workspaceId}/users/{userUin}
    WS->>EG: POST /api/kernels
    EG->>K: 启动 Kernel Pod
    K->>JFS: 挂载当前用户路径
    EG-->>WS: KernelModel
    WS->>DB: 保存 Session + Kernel + LaunchPlan
    WS-->>FE: Jupyter SessionModel
```

### 5.2 WebSocket 代理流程

```mermaid
sequenceDiagram
    participant FE as Notebook UI
    participant WS as Workspace Server
    participant EG as DLC EG

    FE->>WS: WS /api/kernels/{kernelId}/channels
    WS->>WS: 校验 session / kernel ownership
    WS->>EG: WS /api/kernels/{kernelId}/channels
    FE->>WS: execute_request
    WS->>EG: forward
    EG-->>WS: iopub / shell messages
    WS-->>FE: forward (并捕获输出)
```

## 6. Git 操作与文件创建人/更新人记录

需求：用户在**前端**或 **Terminal** 进行 Git 操作时，要把受影响文件的**创建人 / 更新人（主账号、子账号）记录下来。

默认建议仍是把业务身份落到 Studio 业务库；如果产品要求"登录人信息进入 JuiceFS 元数据体系"，需要对 JuiceFS client 做定制改造，让写入发生时就能拿到可信的登录人上下文。下面同时说明两种口径：

- 业务库口径：Workspace Server 写文件时维护 `ide_code_file` / `ide_code_file_version`；
- JuiceFS 元数据口径：改造 JuiceFS client，在 create / write / rename 等文件系统操作中维护登录人扩展表。

### 6.1 身份模型与原则

现有 `RequestContext` 已具备 `appId`、`ownerUin`（主账号）、`uin`（操作子账号）。业务库口径下，最终落库到现有 Studio 文件表的 `owner_uin` / `app_id` / `create_user_uin` / `update_user_uin` 字段；JuiceFS 元数据口径下，则需要把这些字段写入 JuiceFS 扩展表。

**核心原则**：

1. 默认建议业务身份（主/子账号）落 **Studio 业务库**，不依赖 juicefs mysql 的 uid/gid。
2. 如果要求登录人进入 JuiceFS 元数据，不能只改 MySQL 表；必须同步改造 JuiceFS client 的身份上下文传入和元数据写入链路。
3. Terminal 与 Kernel 一样运行用户任意命令，属于**不可信环境**；操作人身份不能靠"信任容器内执行的命令"判定，必须来自 Workspace Server / Pod 启动控制面 / CSI mount 参数等可信边界。

### 6.2 Databricks 的本质：控制平面文件系统，写入即归属

Databricks 之所以不需要 diff / hook，根因是它**自己实现了一套控制平面文件系统**（Workspace files / 对象系统）：所有文件读写都经过它的文件服务，每个写入请求都携带认证后的登录用户。于是——

> **写入即归属**：哪个用户写了哪个文件，在写入发生的那一刻由文件服务直接记录，不存在"事后找操作人"。

由此引出两个**必须分开**的问题：

| 问题 | 答案 |
|---|---|
| 操作人是谁？ | 由文件写入请求的登录身份决定（API 入口即知），**与 diff / hook 无关** |
| 变更了哪些文件？ | 由文件服务的对象同步职责决定。git pull/merge 后，文件服务对比"仓库内容 ↔ 文件对象"得出要新增/更新/删除的对象——这是任何实现（含 Databricks）都要做的同步逻辑，**不是为了追踪身份** |

> 注：上一版把这两件事混在 `git diff` 里表述，造成"靠 diff 找操作人"的误解。实际是：**身份来自入口，变更集合来自同步**。

配套机制：

| 机制 | 说明 |
|---|---|
| 操作走控制平面 | 标准 Git folder 操作通过 UI / Repos API 执行，操作人 = 登录用户，平台直接记录 |
| 凭证即身份 | 用户级 PAT/OAuth，`GIT_AUTHOR/COMMITTER` = 该用户，远端 provider 归属也正确 |
| Folder 归属单一用户 | 每人独立 folder + 分支，folder 内变更天然归属该用户 |

### 6.3 对 WeData 的关键影响：文件写入必须经过 Workspace Server

要复刻"写入即归属"，**文件写入必须经过 Workspace Server 文件服务（携带 `RequestContext` 的 `ownerUin/uin`）**：

- 普通编辑 / 保存：本来就经过 Server → 写入时直接登记 `ide_code_file` 的 `create/update_user_uin`，天然知道登录用户，和 Databricks 一致；
- git 操作：由 Server 执行，再经文件服务把变更同步为对象，**同步写入时登记操作人**；
- 隐患：**git / terminal 直接在裸 JuiceFS 工作树写文件，是绕过 Server 的旁路** → Server 不在写入路径上，才会退化成"事后 diff"。这正是问题根源（裸 JuiceFS 是 POSIX 文件系统，不带业务身份）。

三条路线（需决策）：

| 路线 | 文件写入路径 | 操作人记录 | 代价 |
|---|---|---|---|
| A. 对齐 Databricks / IdentityFS | 前端 Git 走 Workspace Server Git API，go-git 通过 IdentityFS 写 JuiceFS | 写入文件时同步记录 inode -> 登录人，精确 | 仅覆盖受控 Git API，需实现 go-billy adapter |
| B. 裸 JuiceFS 直挂 | kernel/terminal 直接 POSIX 写 JuiceFS | Server 不在写入路径，靠同步对比 + folder/session 归属用户兜底 | 简单、高性能，归属精度不如 A |
| C. 改造 JuiceFS client | 前端 Git / Terminal 仍写 JuiceFS，但 JuiceFS client 能解析登录人上下文 | JuiceFS 写入时直接维护登录人扩展表 | 需要维护 JuiceFS fork，升级和兼容成本高 |

> 建议：前端 Git 优先走路线 A：在 Workspace Server 控制面内执行 go-git，并用 IdentityFS 在逐文件写入时记录登录人。kernel / terminal 运行态对性能敏感的裸写可走 B，其产生的"文件变更人"按 folder/session 绑定用户兜底；只有明确要求 JuiceFS client 自身感知登录人时，再选择路线 C。

### 6.4 前端 Git 操作：go-git IdentityFS 方案

前端 Git 操作的入口在 Workspace Server，因此登录人身份在 API 请求进入时已经可信确定。目标体验是：用户在页面执行 clone / pull 后，文件树里新增文件的创建人立即展示为当前登录人，不能依赖事后 `git diff` 反推。

在当前架构中，Workspace Server 部署时已经挂载了一个统一 JuiceFS 目录，前端 Git 操作没有必要、也不应该每次重新挂载 JuiceFS。此时推荐使用 **go-git filesystem adapter（IdentityFS）**：

> IdentityFS 是一个实现 `go-billy` `Filesystem` 接口的包装层。它内部仍然调用 `osfs` 写统一 JuiceFS mount，但在 create / open / write / close / rename / remove 等文件操作发生时，同步维护 `inode -> 登录人` 的身份索引。

核心链路：

1. 前端调用 Workspace Server 的 Git API（clone / pull / checkout / merge 等）；
2. Workspace Server 从 `RequestContext` 得到 `ownerUin / uin / appId`，可选生成 `operationId` 用于审计和补偿；
3. Workspace Server 创建 `IdentityFS(base=osfs.New(repoPath), actor, identityStore)`；
4. Workspace Server 在当前 HTTP 请求内调用 go-git，worktree filesystem 使用 `IdentityFS`；
5. go-git 执行 pull / checkout 时，每次落工作区文件都会调用 `IdentityFS`；
6. `IdentityFS` 先让底层 `osfs` 写 JuiceFS，成功后 `stat` 文件拿 inode；
7. `IdentityFS` upsert `jfs_wedata_identity`，把 inode 的创建人 / 更新人写为当前登录人；
8. 前端刷新文件树时，通过 path `stat` 得到 inode，再查 `jfs_wedata_identity` 展示创建人 / 更新人。

这不是"git pull 完成后遍历变更文件"，而是"git pull 过程中每个文件系统动作发生时记录身份"。

#### 6.4.1 go-git filesystem adapter 是什么

go-git 的 worktree 不直接依赖本地 `os.Create`，而是依赖 `go-billy` 的 `Filesystem` 抽象。常规用法是把 `osfs.New(repoPath)` 传给 go-git：

```go
baseFS := osfs.New(repoPath)
repo, err := git.Clone(storer, baseFS, opts)
```

IdentityFS 在 `osfs` 外面包一层：

```go
baseFS := osfs.New(repoPath) // repoPath 在统一 JuiceFS mount 下
worktreeFS := NewIdentityFS(baseFS, IdentityOptions{
    RepoRoot:     repoPath,
    OwnerUin:     ctx.OwnerUin,
    Uin:          ctx.Uin,
    AppID:        ctx.AppID,
    WorkspaceID:  workspaceID,
    OperationID:  operationID, // 可选：用于审计、日志关联、补偿重试
    Store:        identityStore,
})

repo, err := git.Clone(storer, worktreeFS, opts)
```

go-git 仍然认为自己在写普通文件系统；区别是所有工作区文件操作会先经过 `IdentityFS`。

#### 6.4.2 git pull 如何执行

已有仓库执行 pull 时，流程可以设计为：

```go
baseFS := osfs.New(repoPath)
worktreeFS := NewIdentityFS(baseFS, opts)

// 推荐把 .git 存储和业务 worktree 分开；如果 .git 也在 repoPath 下，
// IdentityFS 必须过滤 .git/**，避免记录 Git 内部对象文件。
gitStorage := filesystem.NewStorage(osfs.New(gitMetaPath), cache.NewObjectLRUDefault())

repo, err := git.Open(gitStorage, worktreeFS)
if err != nil {
    return err
}

wt, err := repo.Worktree()
if err != nil {
    return err
}

err = wt.Pull(&git.PullOptions{
    RemoteName:    "origin",
    ReferenceName: plumbing.NewBranchReferenceName(branch),
    Auth:          auth,
})
if errors.Is(err, git.NoErrAlreadyUpToDate) {
    return nil
}
return err
```

`wt.Pull` 内部会 fetch 远端对象并更新 worktree。更新工作区文件时，go-git 会调用 `IdentityFS.Create`、`IdentityFS.OpenFile`、`IdentityFS.Rename`、`IdentityFS.Remove`、`IdentityFile.Write`、`IdentityFile.Close` 等方法。身份记录就在这些方法里发生。

#### 6.4.3 IdentityFS 需要拦截哪些方法

IdentityFS 至少需要包装两类接口：

| 接口 | 需要处理的动作 | 记录时机 |
|---|---|---|
| `Filesystem.Create(path)` | 新建文件 | 底层 create 成功后，`stat` path 拿 inode，写 create/update 登录人 |
| `Filesystem.OpenFile(path, flag, perm)` | 打开已有文件或创建文件 | 若 flag 含写入语义，返回 `IdentityFile`；close 时写 update 登录人 |
| `Filesystem.Rename(old, new)` | Git 常见的临时文件 rename | rename 前 `stat old` 记录 inode，rename 成功后更新该 inode 的 update 登录人 |
| `Filesystem.Remove(path)` / `RemoveAll(path)` | 删除文件 | remove 前 `stat` 记录 inode，remove 成功后标记 deleted |
| `Filesystem.MkdirAll(path, perm)` | 创建目录 | 目录也需要展示创建人时，mkdir 成功后 `stat` 目录 inode 并记录 |
| `File.Write` / `File.Truncate` / `File.Close` | 文件内容更新 | write 标记 dirty，close 成功后合并写 update 登录人 |

示意结构：

```go
type IdentityFS struct {
    billy.Filesystem
    repoRoot string
    actor    Actor
    store    IdentityStore
}

type Actor struct {
    OwnerUin    string
    Uin         string
    AppID       string
    WorkspaceID string
    OperationID string
}

type IdentityStore interface {
    UpsertCreatedOrUpdated(ctx context.Context, inode uint64, path string, actor Actor) error
    MarkUpdated(ctx context.Context, inode uint64, path string, actor Actor) error
    MarkDeleted(ctx context.Context, inode uint64, path string, actor Actor) error
}
```

`Create` 示例：

```go
func (fs *IdentityFS) Create(path string) (billy.File, error) {
    f, err := fs.Filesystem.Create(path)
    if err != nil {
        return nil, err
    }
    if fs.shouldRecord(path) {
        fs.recordCreatedOrUpdated(path)
    }
    return &IdentityFile{File: f, fs: fs, path: path, dirty: true}, nil
}
```

`OpenFile` 示例：

```go
func (fs *IdentityFS) OpenFile(path string, flag int, perm os.FileMode) (billy.File, error) {
    f, err := fs.Filesystem.OpenFile(path, flag, perm)
    if err != nil {
        return nil, err
    }

    dirty := flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_TRUNC|os.O_CREATE) != 0
    return &IdentityFile{File: f, fs: fs, path: path, dirty: dirty}, nil
}
```

`IdentityFile` 示例：

```go
type IdentityFile struct {
    billy.File
    fs    *IdentityFS
    path  string
    dirty bool
}

func (f *IdentityFile) Write(p []byte) (int, error) {
    n, err := f.File.Write(p)
    if n > 0 {
        f.dirty = true
    }
    return n, err
}

func (f *IdentityFile) Close() error {
    err := f.File.Close()
    if err == nil && f.dirty && f.fs.shouldRecord(f.path) {
        f.fs.recordUpdated(f.path)
    }
    return err
}
```

`Rename` 示例：

```go
func (fs *IdentityFS) Rename(oldPath, newPath string) error {
    inode, _ := fs.lookupInode(oldPath)
    if err := fs.Filesystem.Rename(oldPath, newPath); err != nil {
        return err
    }
    if inode > 0 && fs.shouldRecord(newPath) {
        _ = fs.store.MarkUpdated(context.Background(), inode, newPath, fs.actor)
    }
    return nil
}
```

`recordUpdated` 的关键是先让 JuiceFS 完成写入，再通过 `stat` 拿 inode：

```go
func (fs *IdentityFS) recordUpdated(path string) {
    inode, err := fs.lookupInode(path)
    if err != nil {
        fs.enqueueRetry("update", path)
        return
    }
    if err := fs.store.MarkUpdated(context.Background(), inode, path, fs.actor); err != nil {
        fs.enqueueRetry("update", path)
    }
}
```

#### 6.4.4 inode 获取时序

如果 `jfs_wedata_identity` 以 inode 为主键，确实必须先写 JuiceFS，再拿 inode：

```text
Create/OpenFile/Rename/Remove
  -> 调底层 osfs 写统一 JuiceFS mount
  -> 写入成功
  -> os.Stat / File.Stat 获取 inode
  -> upsert jfs_wedata_identity
```

这不是事后扫描，因为它发生在每个文件操作回调里。对于 `git pull`，一个文件被更新时，IdentityFS 会在该文件 close / rename 成功后立即记录；pull 还没整体结束时，已处理文件的身份记录已经可以写入。

#### 6.4.5 性能优化项

`operationId` 不是记录创建人 / 更新人的必需字段。若只需要展示文件当前创建人 / 更新人，`ownerUin / uin / appId` 已足够；`operationId` 主要用于审计、日志关联、幂等和补偿重试，建议作为可选字段保留。

IdentityFS 相比普通 `osfs` 的额外成本主要是：

```text
每个变更文件：stat 获取 inode + 写入 jfs_wedata_identity
```

为避免大仓库 / 大量小文件场景明显变慢，建议：

- 不在每次 `Write` 时写身份表，只在 `Create`、`Close`、`Rename`、`Remove` 等文件级动作完成后记录；
- `.git/**` 内部文件默认不记录；
- `stat` 后先写入内存 buffer，按批次 upsert `jfs_wedata_identity`；
- DB 写失败进入 outbox / retry，不在非关键场景阻塞整个 Git 操作；
- 同一个 repo 同一时间只允许一个 `pull / checkout / merge`，避免并发写同一 worktree；
- 对 clone 大仓库可增加批量大小、最大重试次数、慢操作日志等观测指标。

粗略预期：大文件少量变更开销较小；几百个文件变更时如果逐条同步 upsert，可能带来明显延迟；大量小文件场景必须依赖批量 upsert 和异步补偿。

#### 6.4.6 需要过滤和补偿的场景

IdentityFS 需要显式处理以下边界：

| 场景 | 处理建议 |
|---|---|
| `.git/**` 内部文件 | 默认不记录；否则 objects、refs、index 会产生大量无业务意义的身份记录 |
| 临时文件 + rename | 以 inode 为准更新操作人；路径展示由当前文件树根据 path `stat` 后反查身份 |
| DB 写失败 | 不阻塞 Git 主流程或按产品要求失败回滚；推荐写 outbox/retry，按 path 重新 stat inode 后补偿 |
| hardlink | 若后续支持 hardlink，身份以 inode 为准；同一 inode 可能对应多个路径，路径不进入身份索引 |
| 删除文件 | remove 前 stat inode，remove 成功后标记 deleted；是否保留审计由产品决定 |
| 并发 Git 操作 | 同一个 repoObjectId 同一时间只允许一个 pull/checkout/merge，避免 worktree 并发写 |

#### 6.4.7 方案边界

IdentityFS 只覆盖 **前端 Git API / Workspace Server 调用 go-git** 的路径。用户在 Terminal 中直接运行原生 `git pull`，不会经过 IdentityFS，因此不能用它记录逐文件创建人/更新人。Terminal 仍需走 6.5 的 Pod / session 归属兜底，或走 6.8 的 JuiceFS client 改造。

```mermaid
sequenceDiagram
    participant FE as Git UI
    participant WS as Workspace Server
    participant GG as go-git
    participant IFS as IdentityFS<br/>(go-billy adapter)
    participant JFS as 统一 JuiceFS mount
    participant Remote as Git Provider
    participant Meta as JuiceFS MySQL<br/>扩展表

    FE->>WS: Git 操作(repoId, op)<br/>登录人从 RequestContext 确定
    WS->>IFS: 创建 IdentityFS<br/>actor + operationId + repoPath
    WS->>GG: 调用 go-git Pull/Clone<br/>worktreeFS=IdentityFS
    GG->>Remote: fetch / pull
    GG->>IFS: create/open/write/rename/remove
    IFS->>JFS: 调底层 osfs 写 JuiceFS
    IFS->>IFS: stat path 获取 inode
    IFS->>Meta: upsert inode 的 create/update 登录人
    WS-->>FE: 操作结果
```

### 6.5 Terminal Git 操作

Terminal 直接在裸 JuiceFS 工作树写文件，不经过 go-git IdentityFS，因此不能复用 6.4 的逐文件拦截能力。这里的 Terminal 是用户连接 DLC 引擎后打开的 **driver pod** 终端；DLC 会启动 driver pod 和 executor pod，前端 Terminal 连接的是 driver pod。当前确定前提是：**一个 driver pod 只属于一个登录用户**，且 driver pod 环境变量中已有当前登录人的主账号、子账号信息。

#### 6.5.1 身份来源

driver pod 由 DLC 引擎启动，启动参数 / 环境变量中已经带有登录人信息。Terminal 进入 driver pod 后，操作人以 driver pod 绑定身份为准：

- `ownerUin`：主账号；
- `uin`：子账号 / 操作人；
- `appId`：主账号 appId；
- `workspaceId`：当前 workspace；
- `driverPodName` / `driverPodUid`：driver pod 标识；
- `kernelId` / `sessionId`：Notebook / Kernel 会话标识（如有）。

这些信息来自 driver pod 环境变量，而不是 Terminal 进程自行声明。变量名以实际 DLC 注入为准，示例：

```text
WEDATA_OWNER_UIN=100001
WEDATA_UIN=200001
WEDATA_APP_ID=app-xxx
WEDATA_WORKSPACE_ID=ws-xxx
WEDATA_KERNEL_ID=kernel-xxx
WEDATA_DRIVER_POD_UID=pod-xxx
```

需要注意：用户在 shell 里可以修改当前进程的环境变量，因此 Workspace Server 不能信任 Terminal 命令上报的环境变量值。可信做法是：Workspace Server / 同步 worker 从 Kubernetes Pod spec、DLC 会话元数据或启动时登记的 driver pod 绑定关系读取身份；driver pod 环境变量只作为 driver pod 身份的来源之一，不能由用户请求直接透传。

#### 6.5.2 首期方案：按 driver pod 归属记录

首期方案不追求像 IdentityFS 那样拦截每一次 `create/write/rename`。Terminal 内运行的是 driver pod 里的原生 `git`，它直接写统一 JuiceFS mount；Workspace Server 不在写入调用栈里，因此不能实时知道每个文件操作。首期采用 **driver pod 单登录人 + dirty scope 同步**：

```text
操作人来源：driver pod 绑定的登录人
变更范围来源：前端/Workspace Server 触发，或 driver pod 写入 JuiceFS dirty marker
文件集合来源：同步 worker 扫描 dirty scope 后得到的最终文件状态
身份落点：jfs_wedata_identity
```

完整流程：

1. **连接 DLC 引擎并启动 driver pod**
   - 用户在前端连接 DLC 引擎 / 创建 Kernel；
   - DLC 启动 driver pod 和 executor pod；
   - driver pod 环境变量中注入 `ownerUin / uin / appId / workspaceId` 等登录身份；
   - Workspace Server 记录 driver pod 与登录人的绑定关系：

     ```text
     driverPodUid -> ownerUin / uin / appId / workspaceId / userRootPath
     ```

   - `userRootPath` 通常是：

     ```text
     /{tenantId}/{workspaceId}/users/{userUin}/
     ```

2. **打开 driver pod Terminal**
   - 用户在前端打开 Terminal；
   - Workspace Server / 网关将 Terminal 连接到该用户对应的 driver pod；
   - driver pod 内配置该用户的 Git credential、`user.name`、`user.email`；
   - driver pod 的身份来自 DLC 启动环境与 Workspace Server 记录的绑定关系，不依赖用户在 shell 内自行声明。

3. **用户执行原生 Git 命令**
   - 用户在 Terminal 中执行：

     ```bash
     cd /workspace/users/200001/my-repo
     git pull
     ```

   - 原生 Git 直接写 JuiceFS 工作树；
   - 此时不会经过 IdentityFS，也不会逐文件实时写 `jfs_wedata_identity`。

4. **产生 dirty scope**
   - driver pod 不能访问 Workspace Server，因此不能依赖 shell hook / Git wrapper / inotify agent 直接调用 Workspace Server；
   - 可行的 dirty scope 来源有两类：
     - 前端 / Workspace Server 侧触发：文件树刷新、Terminal WebSocket 断开、Kernel/driver pod 空闲或停止；
     - driver pod 侧写共享标记：shell hook / Git wrapper / inotify agent 只把 dirty marker 写到 JuiceFS 上的受控目录，Workspace Server 后续读取该 marker。
   - dirty marker 内容示例：

     ```text
     driverPodUid = pod-xxx
     pathPrefix = /{tenantId}/{workspaceId}/users/{userUin}/my-repo
     reason = git_wrapper | shell_hook | inotify | refresh | idle
     ```

   - 这里的 `pathPrefix` 是"需要做身份补齐的路径前缀"，不是变更文件列表。

5. **Workspace Server 入队并合并**
   - Workspace Server 根据 `driverPodUid` 查到绑定登录人与 `userRootPath`；
   - Workspace Server 校验 `pathPrefix` 必须在该 driver pod 绑定的 `userRootPath` 下；
   - 将前端触发的 dirty scope 或 JuiceFS dirty marker 写入 `terminal_dirty_scope` 去重队列；
   - 同一 `driverPodUid + pathPrefix` 在 debounce 窗口内合并为一个任务；
   - 如果 dirty scope 是子路径，服务端可以向上合并到 Git repo 根目录，避免一次 `git pull` 产生大量扫描任务。

6. **同步 worker 扫描 dirty scope**
   - 有限数量 worker 消费队列；
   - worker 读取 `driverPodUid` 对应的登录人；
   - 扫描 `pathPrefix` 下的文件树，跳过 `.git/**`；
   - 对每个文件 `stat` 获取 inode；
   - 和 `jfs_wedata_identity` 中已有记录比较，识别新增、更新、删除。

7. **写入身份表**
   - 新文件：

     ```text
     create_user_uin = driverPod.uin
     update_user_uin = driverPod.uin
     ```

   - 已存在但内容、mtime、size 或 hash 变化：

     ```text
     update_user_uin = driverPod.uin
     ```

   - 删除文件：

     ```text
     deleted = 1
     update_user_uin = driverPod.uin
     ```

这个方案的语义是：**driver pod 中发生的文件变化，归属该 driver pod 绑定用户**。它不是强审计级别的逐 syscall 归因，而是基于单用户 driver pod 的最终状态 reconciliation。对用户个人目录下的 Git 仓库，这是可接受的；对多人共享目录或共享 repo，需要额外加锁或降低精度预期。

#### 6.5.3 文件同步与身份表写入

Terminal Git 由于绕过 IdentityFS，无法在每个 `create/write/rename` 回调里实时拿到 inode。又因为 driver pod 不能访问 Workspace Server，首期不能依赖 driver pod 主动上报服务端；dirty scope 必须由 Workspace Server 侧触发，或由 driver pod 写入 JuiceFS 上的 marker 后由 Workspace Server 拉取：

| 触发方式 | 说明 | 适用性 |
|---|---|---|
| 文件树刷新时按需同步 | 前端打开/刷新目录时，Workspace Server 直接把当前目录作为 dirty scope 入队 | 用户感知直接，首期推荐 |
| Terminal 关闭 / driver pod 空闲同步 | 前端断开 Terminal，或 Workspace Server 发现 driver pod 空闲/停止后，将该用户最近工作目录入队 | 简单，适合兜底 |
| 定时后台同步 | Workspace Server 周期扫描活跃 driver pod 绑定的用户目录或 repo 根目录 | 兜底，不应深度全量扫描所有目录 |
| Shell hook / PROMPT_COMMAND 写 marker | 每条命令结束后，driver pod 把当前目录写入 JuiceFS dirty marker 文件，Workspace Server 后续读取 marker 入队 | 不需要访问 Workspace Server，但用户可绕过 |
| Git wrapper 写 marker | 用 wrapper 包装 `git` 命令，执行后把 Git repo 根目录写入 JuiceFS dirty marker | 更及时，能识别 repo 根目录，但需限制用户绕过 wrapper |
| inotify sidecar / agent 写 marker | driver pod 内监听用户目录文件事件，合并后写 JuiceFS dirty marker | 覆盖更广，但事件量、rename、队列溢出要处理 |

服务端不应为每次命令启动一个独立扫描任务，而应使用去重队列：

```text
terminal_dirty_scope
- workspace_id
- driver_pod_uid
- kernel_id / session_id
- actor_owner_uin
- actor_uin
- path_prefix
- reason(shell_hook / git_wrapper / inotify / refresh / idle)
- status(pending / running / done / failed)
- last_event_time
- next_run_time
```

dirty marker 可放在 Workspace Server 可读、用户不可随意伪造的受控路径。若只能放在用户可写路径，服务端必须校验 marker 中的 `path_prefix` 在该 driver pod 绑定的 `userRootPath` 下，并忽略 marker 中自带的用户身份，操作人始终以 driver pod 绑定关系为准。

合并规则：

- 同一 `driverPodUid + path_prefix` 在 debounce 窗口内只保留一个 pending 任务；
- 子路径 dirty scope 可向父 repo 根目录合并，避免同一 Git 操作产生大量任务；
- worker 数量全局限流，按租户 / workspace 做并发控制；
- 文件树刷新只同步当前目录或有限深度，深度补齐交给后台 worker；
- 多次同步同一 inode 时，`update_user_uin` 以最后一次成功同步的 actor 为准。

同步任务写 `jfs_wedata_identity` 的规则：

- 新文件：`create_user_uin / update_user_uin = driverPod.uin`；
- 已存在但内容或 mtime 变化：`update_user_uin = driverPod.uin`；
- 删除文件：标记 `deleted = 1`，可选记录 `update_user_uin = driverPod.uin`；
- `.git/**` 默认不写身份索引；
- 如果只能拿到 path，先 `stat` path 得到 inode，再 upsert 身份表。

并发约束：

- 用户通常在 `/{tenantId}/{workspaceId}/users/{userUin}/` 下 clone 仓库，不同用户路径天然隔离；
- 若允许多个 driver pod 写同一个共享目录或同一个 repo，需要 repo/path 级写锁，否则事后目录同步无法精确区分同一文件由哪个 driver pod 最后写入；
- 共享路径若不加锁，只能记录"最后同步归属"，不能作为强审计。

#### 6.5.4 可选增强：Terminal Git wrapper

如果希望 Terminal 内执行 Git 命令后更快更新身份索引，可以提供 Git wrapper：

```text
git -> wedata-git-wrapper -> /usr/bin/git
```

wrapper 做三件事：

1. 读取 driver pod 环境变量中的 `ownerUin / uin / appId / workspaceId`，或读取 Workspace Server 登记的 `driverPodUid -> 登录人` 绑定；
2. 执行真实 Git 命令；
3. 命令结束后把 repo 根目录写入 JuiceFS dirty marker；Workspace Server 后续读取 marker 并同步该目录，服务端以 driver pod 绑定用户作为操作人。

该方案仍不是逐 `write` 实时记录，但比纯后台扫描及时，且不要求 driver pod 访问 Workspace Server。风险是用户可能绕过 wrapper 直接执行 `/usr/bin/git`；若要强制生效，需要控制 PATH、文件权限或 driver pod 基础镜像。

#### 6.5.5 可选增强：改造 JuiceFS client 做实时记录

如果要求 Terminal 中原生 `git pull` 也做到和前端 IdentityFS 一样的逐文件实时记录，只能让 JuiceFS client 自身识别 driver pod 的登录人：

1. driver pod 单登录人绑定；
2. JuiceFS mount 在 driver pod 内执行时，启动参数读取 DLC 注入的登录人环境变量，将该 mount 的默认登录人设为该用户；
3. 如果 JuiceFS 通过 CSI 在 node 层挂载，node 层 mount 默认读不到 driver 容器内环境变量，必须通过 CSI volume attribute / mount option 把登录人传给 `juicefs mount`；
4. 改造 JuiceFS client，在 create / write / rename / flush / release 等路径写 `jfs_wedata_identity`。

这个方案实时性最好，但需要维护 JuiceFS fork，且和 CSI 挂载、mount 生命周期、租户隔离强相关，建议作为增强项，不作为首期主路径。

#### 6.5.6 方案边界

- driver pod 单登录人是该方案成立的前提；
- Terminal 内所有写入都默认归属 driver pod 绑定用户，不区分是否是 Git 命令、Python 脚本或编辑器写入；
- 用户目录下的 repo 由路径天然隔离；若同一个 workspace 允许多个 driver pod 同时写同一共享 repo，需要 repo/path 级写锁，否则只能记录最后同步归属；
- 若用户在 Terminal 里修改当前 shell 环境变量，不影响归属；服务端应以 driver pod spec / DLC 会话元数据中的身份为准；
- 前端 Git 仍优先走 6.4 IdentityFS，Terminal 是另一条旁路写入路径。

#### 6.5.7 Databricks 参考：Terminal / Notebook 生成文件

Databricks 的公开文档体现出几个关键取舍：

| 现象 | Databricks 行为 | 对 WeData 的启发 |
|---|---|---|
| Workspace Files | 支持用标准 Python `os` / `shutil`、shell 命令创建、更新、删除 workspace files | 运行时代码生成文件可以进入工作区文件系统，但必须受 workspace 权限与配置控制 |
| Notebook CWD | Databricks Runtime 14.0+ 默认 CWD 是当前 notebook / script 所在目录；旧版本或关闭 WSFS 时可能是 driver ephemeral storage | Notebook 生成文件是否持久化，取决于 CWD 是否落在 workspace 文件系统；WeData 也应显式定义 CWD |
| Git folders | Databricks Git folders 是受控 workspace 对象；普通 workspace files 不允许创建 `.git` 目录 | Git 能力不是普通裸文件系统能力，平台需要区分受控 Git 目录和普通 workspace 文件 |
| Web terminal Git CLI | Git CLI support 是单独能力；通过 web terminal clone 的 Git folder 可获得 CLI access，但 Repos API 对 CLI-enabled Git folder 有限制 | Terminal Git 是受控能力开关，不应默认等同于前端 Git API |
| 临时文件 | Databricks 建议临时/大文件写 `$TEMPDIR` 或其他数据位置，避免 workspace filesystem 性能和容量限制 | Notebook / Terminal 生成的大数据产物不应默认写入 workspace 根目录 |

据此，WeData 可以参考以下规则：

1. **Notebook / Python 默认 CWD 明确化**：Notebook 运行时默认 CWD 可以设置为 notebook 所在目录；如果用户在该目录下写文件，则文件持久化到 JuiceFS，并按 Notebook session 绑定用户记录操作人。
2. **临时与大文件分流**：提供明确的临时目录（如 `/tmp` 或 workspace 下的 runtime scratch），引导模型中间结果、缓存、训练输出不要写入代码目录；这类文件可不进入文件树创建人/更新人展示。
3. **Notebook / driver pod 单登录人绑定**：一个 Notebook session / Kernel 对应的 DLC driver pod 绑定登录用户；该 driver pod 直接写 JuiceFS 的文件，首期按 session / driver pod 归属用户记录。
4. **受控 Git 与 Terminal Git 分层**：前端 Git API 走 IdentityFS，能逐文件记录；Terminal Git / Notebook Python 直接写文件属于旁路写入，首期按 driver pod / session 归属 + 同步任务记录。
5. **权限与审计不依赖 Linux owner**：Databricks Workspace Files 以 workspace ACL / object metadata 管理权限；WeData 也不应依赖 JuiceFS uid/gid 表达业务登录人。

Notebook / Python 生成文件的推荐处理方式：

| 写入来源 | 推荐归属 | 记录方式 |
|---|---|---|
| Notebook 在自身目录写小文件 | Notebook session 绑定用户 | session 结束 / 文件树刷新 / 定时同步后写 `jfs_wedata_identity` |
| Notebook 写临时目录 | 可不展示创建人，或按 session 绑定用户做审计 | 不进入普通文件树，按生命周期清理 |
| Notebook 写 Git repo 工作区 | Notebook session 绑定用户，但需提示这可能产生未提交变更 | 同步任务写 update_user；前端 Git 后续可通过 IdentityFS 处理受控 Git 操作 |
| Spark executor 写文件 | 不建议写 workspace files；应写数据存储 / volume / 对象存储 | 不纳入 workspace 文件创建人模型 |

```mermaid
sequenceDiagram
    participant FE as Frontend
    participant WS as Workspace Server
    participant DLC as DLC
    participant Driver as Driver Pod
    participant JFS as 统一 JuiceFS mount
    participant Queue as Dirty Scope Queue
    participant Sync as 同步 Worker
    participant Meta as JuiceFS MySQL<br/>扩展表

    FE->>WS: 连接 DLC 引擎 / 打开 Terminal
    WS->>DLC: 创建 Kernel / 启动作业
    DLC->>Driver: 启动 driver pod<br/>注入登录人环境变量
    WS->>WS: 记录 driverPodUid -> 登录人
    Driver->>JFS: 用户执行 git clone / pull<br/>产生 create/write/rename
    Driver->>JFS: shell hook / wrapper / inotify<br/>写 dirty marker
    FE-->>WS: 文件树刷新 / Terminal 关闭<br/>触发当前目录 dirty scope
    WS->>JFS: 读取 dirty marker
    WS->>Queue: 按 driverPodUid + path_prefix 去重合并
    Queue->>Sync: 有限 worker 消费
    Sync->>JFS: 扫描 dirty scope，stat 文件获取 inode
    Sync->>Meta: upsert inode 的 create/update 登录人<br/>= driver pod 绑定用户
```

### 6.6 业务库表与字段（基于现有 OneFlow Studio 表）

现有 Studio 文件相关表**已具备创建人/更新人字段**，记录文件创建人/更新人无需新建表，直接复用：

`ide_code_file`（文件主表，最新态）

| 字段 | 说明 |
|---|---|
| code_file_id | 文件 uuid |
| create_user_uin | 创建人子账号 uin（已存在） |
| update_user_uin | 更新人子账号 uin（已存在） |
| owner_uin | 主账号 uin（已存在） |
| app_id | 主账号 appId（已存在） |
| workspace_id / status / extension_type | 工作空间、状态、类型等 |

`ide_code_file_version`（版本表，历史态，逐版本记录操作人）

| 字段 | 说明 |
|---|---|
| version_id / version | 版本 uuid 与递增版本号 |
| code_file_id | 关联 `ide_code_file.code_file_id` |
| create_user_uin / update_user_uin / owner_uin / app_id | 该版本由谁产生（均已存在） |

> 结论：文件级"创建人/更新人"用 `ide_code_file`（最新态）+ `ide_code_file_version`（历史态）即可覆盖。**Git 操作只是这些字段的又一个写入来源**——git 改了哪些文件，就把对应 `ide_code_file` 的 `create_user_uin/update_user_uin` 更新为执行该 git 操作的人；普通编辑保存时本来也会写这两个字段。

可选（仅当需要 git 维度追溯"哪次 commit 改了哪些文件、对应操作人"时）新增轻量表，但它**不是记录创建人/更新人的必需项**：

| 表 | 关键字段 |
|---|---|
| `ide_git_operation`（可选） | operation_id、source(frontend/terminal)、operation_type、owner_uin、sub_uin、app_id、workspace_id、repo_id、commit_id、branch、result、create_time |

### 6.7 JuiceFS MySQL 是否要新增创建人/更新人字段

**不建议在 juicefs mysql 中新增业务创建人/更新人字段。** 原因：

| 角度 | 说明 |
|---|---|
| schema 是 JuiceFS 私有 | juicefs mysql 的表（`jfs_node` / `jfs_edge` 等）由 JuiceFS client 维护，结构随版本演进，自行加列属于改动私有 schema |
| client 不维护新列 | JuiceFS client 不认识新增的业务列，写文件时不会填、读时不会用，等于死字段，需业务侧旁路双写，复杂且易不一致 |
| 升级冲突 | JuiceFS 版本升级 / 元数据迁移可能与自定义列冲突 |
| 语义错位 | juicefs 只有 uid/gid 表达"人"，是 POSIX 概念；业务"主账号 + 子账号"是二维身份，无法塞进 uid/gid |

**如果确实要"按 inode/路径"维度记录归属（仍不推荐改 juicefs 表），两种方式：**

| 方式 | 做法 | 代价 |
|---|---|---|
| 旁路扩展表（相对可接受） | 不改 juicefs 自身表，新建业务侧映射表，以 inode 或路径为 key 记录 create_user_uin/update_user_uin，由 Workspace Server 每次写文件时同步维护 | 业务侧双写，需处理 inode 复用 / 路径变更 |
| 直接改 jfs_node 等表加列（强不推荐） | 加列并改造 client 写入路径 | 侵入私有 schema，升级即可能失效 |

> 推荐：juicefs 原生表保持原生；如果必须把登录人放进 JuiceFS 元数据体系，也优先新增扩展表，不直接改 `jfs_node` / `jfs_edge` 等原生表。

**附：用 xattr（扩展属性）记录操作人？——不建议作为权威来源**

JuiceFS 支持 xattr（存在元数据库 `jfs_xattr` 表），看似可把操作人附着到文件。但它只是换了存储位置，**没有解决"谁可信地写 + 身份从哪来"两个根本问题**，且自带坑：

| 问题 / 坑 | 说明 |
|---|---|
| 谁可信地写 | 裸挂载下由用户进程写，用户对 `user.*` xattr 有写权限可伪造/篡改，不能作权威审计；`git pull/checkout` 不会自动写 xattr |
| 身份从哪来 | 写 xattr 仍需先知道操作人；统一 uid 的挂载进程并不知道业务子账号 |
| 原子保存丢失 | 编辑器 / Jupyter 常用"写临时文件 + rename"保存，xattr 不跟随；`cp`/`tar` 默认也不保留 → 操作人丢失 |
| 难查询 | "列出某人改过的文件"需遍历 `getfattr`，无法像 DB 那样建索引 |
| 防篡改 / 性能 | `user.*` 可被有写权限者修改；每次写多一次元数据操作 |

> 结论：xattr 不适合作为权威索引。即使采用 JuiceFS 元数据改造，登录人也应写入可索引的扩展表；xattr 只适合做文件旁挂展示信息。

### 6.8 可选增强：改造 JuiceFS client 写入登录人

这里的 **JuiceFS client** 指 `juicefs mount` 启动的用户态文件系统进程。应用进程（Workspace Server / DLC driver pod 内 git）对挂载目录执行普通 POSIX 文件操作，Linux kernel 把这些操作转成 FUSE 请求，交给 `juicefs mount` 进程处理；`juicefs mount` 再读写 JuiceFS 元数据引擎（MySQL）和对象存储（COS）。

因此，如果确定要让 JuiceFS 自身在写入文件时记录登录人，不能只改 MySQL 表结构，还必须改 `juicefs mount` 对应的源码链路。

#### 6.8.1 需要改造的位置

JuiceFS 官方源码分层大致是：

- `cmd`：命令入口，`juicefs mount` 的参数解析和启动在这一层；
- `pkg/fuse`：FUSE 适配层，能拿到 kernel FUSE request 的 `pid / uid / gid`；
- `pkg/vfs`：VFS 实现层，处理 create / mkdir / open / write / rename / setattr / flush / release 等文件系统语义；
- `pkg/meta`：元数据引擎层，包含 SQL / Redis / TKV 等 metadata engine 的读写实现。

改造点按职责分三层：

| 层次 | 改造点 | 说明 |
|---|---|---|
| mount 参数入口 | 在 `cmd` 层新增 WeData 私有 mount 参数，例如 `--wedata-owner-uin`、`--wedata-uin`、`--wedata-app-id`、`--wedata-workspace-id` | 支持"登录人绑定 mount 实例"，让该 mount 下所有写入默认归属同一登录人 |
| FUSE 请求入口 | 在 `pkg/fuse/context.go` 一类的 FUSE context 中读取 FUSE request 的 `pid / uid / gid` | 若采用子进程 / 进程池方案，可用 `pid/pgid` 反查本次 Git 操作登录人；mount 绑定方案可不依赖 pid |
| VFS 写入入口 | 在 `pkg/vfs/vfs.go`、`pkg/vfs/handle.go`、`pkg/vfs/writer.go` 相关 create / open / write / rename / flush / release 路径接入身份解析 | create 时写创建人；文件内容变更完成时合并更新更新人；避免每个小块 write 都同步落库 |
| 元数据引擎层 | 在 `pkg/meta` 的 SQL metadata engine 中新增 WeData 扩展表读写接口 | 将 inode 的 create/update 登录人写到 `jfs_wedata_identity` |

实现上应在 JuiceFS client 内部新增一个小的 identity 模块，而不是把登录人逻辑散落在各个文件操作函数里。推荐接口形态：

```go
type Identity struct {
    OwnerUin    string
    Uin         string
    AppID       string
    WorkspaceID string
    OperationID string
}

type IdentityResolver interface {
    Resolve(ctx meta.Context, inode Ino, op string) (Identity, bool)
}
```

元数据写入应和 inode 创建/更新尽量保持同一事务或同一重试语义，避免文件已创建但身份表漏写。

#### 6.8.2 扩展表建议

新增表比修改 `jfs_node` 更稳妥：

```sql
CREATE TABLE jfs_wedata_identity (
  inode BIGINT NOT NULL PRIMARY KEY,
  create_owner_uin VARCHAR(64),
  create_user_uin VARCHAR(64),
  update_owner_uin VARCHAR(64),
  update_user_uin VARCHAR(64),
  create_operation_id VARCHAR(64),
  update_operation_id VARCHAR(64),
  deleted TINYINT NOT NULL DEFAULT 0,
  create_time DATETIME,
  update_time DATETIME,
  KEY idx_create_user (create_user_uin),
  KEY idx_update_user (update_user_uin),
  KEY idx_update_time (update_time)
);
```

查询方式：

- 查单个文件的登录人：先对路径 `stat` 得到 inode，再查 `jfs_wedata_identity`；
- 查某个登录人创建/更新的文件：走 `idx_create_user` / `idx_update_user`；
- 展示路径：以当前目录树为准，通过 path `stat` 得到 inode 后查询身份；已删除文件仅保留 inode 级删除标记，不在身份表中维护路径快照。

不要直接给 `jfs_node` 加 `create_user_uin / update_user_uin` 字段。`jfs_node` 是 JuiceFS 原生 schema，升级、迁移、不同 metadata engine 兼容都会受影响；而且登录人字段是 WeData 业务扩展，不应该污染 JuiceFS 核心 inode 表。

#### 6.8.3 前端 Git 与 JuiceFS client 改造的关系

前端 Git 的首选方案是 6.4 的 IdentityFS：身份记录发生在 Workspace Server 的 go-git 文件系统适配层，不要求 JuiceFS client 理解 HTTP 登录人。

只有在产品明确要求"所有 POSIX 写入都必须由 JuiceFS client 统一写身份表"时，才考虑本节的 JuiceFS client 改造。此时必须给 JuiceFS client 一个它能识别的身份边界：

| 身份边界 | 做法 | 适用性 |
|---|---|---|
| mount 维度 | 为登录人 / workspace 使用独立 mount，并在 mount 参数中绑定 `ownerUin / uin / appId` | 能让 JuiceFS client 自动记录身份，但 mount 数量和生命周期管理成本高 |
| 进程维度 | Git Worker 进程池执行 go-git，执行前注册 `pid/pgid -> operationId -> ownerUin/uin` | 可在单一 mount 下区分操作人，但必须保证 worker 进程同一时刻只服务一个登录人 |
| uid 维度 | 每个登录人映射一个 Linux uid，写入进程使用对应 uid | 适合强隔离运行时，但业务主账号 + 子账号仍需额外映射表 |

不推荐"单一 mount + Workspace Server goroutine + JuiceFS client 读取某个协程身份文件"。FUSE request 没有 goroutine 信息，JuiceFS client 无法可靠知道当前写入属于哪个 HTTP 请求。

#### 6.8.4 Terminal 的身份绑定

Terminal 方案可以采用 driver pod 级身份绑定：

1. DLC 启动 driver pod 时注入登录人环境变量（主账号、子账号、appId、workspaceId 等）；
2. 如果 JuiceFS mount 在 driver pod 内执行，JuiceFS client 启动时读取这些环境变量，并将该 mount 的默认登录人设为该用户；
3. 如果 JuiceFS 通过 CSI 在 node 层挂载，则不能依赖 driver 容器内环境变量，需要通过 CSI volume attribute / mount option 把登录人传到 node 层 mount；
4. 该 driver pod 内所有文件写入都归属该 driver pod 绑定的登录人。

driver pod 内环境变量可以作为身份来源，但服务端校验与归属应以 driver pod spec / DLC 会话元数据为准，不信任用户在 shell 中修改后的环境变量值。

### 6.9 小结

核心：对齐 Databricks 的"**控制平面文件系统，写入即归属**"——操作人由前端 API / Pod 启动控制面在入口确定，文件创建/更新发生时就写入身份元数据，不靠事后 `git diff` / hook 反推。

- **前端 Git**：推荐当前 HTTP 请求内直接调用 go-git，并使用 IdentityFS 包装 go-billy filesystem；IdentityFS 写统一 JuiceFS mount 后 `stat` inode，再写 `jfs_wedata_identity`；
- **不是事后 diff**：IdentityFS 在 create / open / write / close / rename / remove 这些文件操作回调里记录身份，不需要等 pull 完成后遍历变更文件；
- **不要用 goroutine 身份文件**：FUSE request 不携带 goroutine 信息，`/wedata/{协程}/info` 无法被 JuiceFS client 可靠关联；
- **Terminal**：Terminal 打开的是 DLC driver pod；driver pod 环境变量中已有登录人信息。若 driver pod 不能访问 Workspace Server，首期按 driver pod / session 归属记录操作人，由文件树刷新、Terminal 关闭、后台扫描触发同步；shell hook / Git wrapper / inotify 只能写 JuiceFS dirty marker，再由 Workspace Server 读取 marker、去重合并后写 `jfs_wedata_identity`；
- **Notebook / Python 生成文件**：参考 Databricks Workspace Files，默认 CWD 可指向 notebook 所在目录；小文件按 Notebook session 绑定用户归属，大文件/临时文件引导写临时目录或数据存储，不进入普通 workspace 文件创建人模型；
- **commit 归属**：统一用用户级 git 凭证（`GIT_AUTHOR/COMMITTER` = 操作人），远端 provider 也可追溯；
- **JuiceFS 表结构**：新增 `jfs_wedata_identity` 扩展表，不直接改 `jfs_node`；xattr 不作为权威索引。

## 7. 演进路径

| 阶段 | 工作 |
|---|---|
| 1. 存储打通 | 部署共享 JuiceFS（MySQL + COS），Workspace Server 与测试 Kernel Pod 双端挂载验证路径隔离 |
| 2. 文件链路切换 | Workspace Server 接管文件 CRUD，直接读写 JuiceFS，收敛 OneFlow 文件创建链路 |
| 3. Kernel 链路切换 | Workspace Server 生成 `KERNEL_*` / `KERNEL_MOUNT`，对接 DLC EG，代理 WebSocket |
| 4. 个人 Pod 下线 | 去掉 code-server / fuse-proxy，停止为用户创建个人 Pod，CFS 数据迁移到 JuiceFS |

## 8. 待决策项

| 待决策 | 说明 |
|---|---|
| JuiceFS 元数据库选型与容量 | 单库承载全部租户 inode，需评估 MySQL 规格、分库与高可用 |
| Kernel 挂载方式 | 确认 CSI Driver 方案，subPath 限定与 mount pod RBAC 隔离细节 |
| 隔离强度是否够用 | 共享实例是否满足合规要求，是否需要向 Per-Tenant JuiceFS 演进 |
| Git 凭证签发与代理 | Credential Helper、Git 代理、出口管控的落地形态与性能 |
| CFS → JuiceFS 迁移 | 存量数据迁移与摘要校验方案 |
| Web IDE 编辑能力 | 去掉 code-server 后，编辑能力如何由前端 + Workspace Server 承接 |
