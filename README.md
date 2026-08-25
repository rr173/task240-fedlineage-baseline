# 联邦学习模型更新谱系一致性服务（task240-fedlineage）

校验多轮联邦聚合中客户端更新的合法祖先关系：模型参数摘要、轮次父节点与
聚合输入的一致性。服务不调度训练，只校验每个数学更新的合法祖先关系，并
在可聚合后发布不可变谱系快照。

## 业务闭环

1. 登记全局模型轮次（preparing → receiving）。
2. 客户端并行上报更新（幂等去重，重放判 replay）。
3. 停止接收（validating），逐更新校验父模型关系与参数形状。
4. 分叉/重放更新隔离；其余形成可聚合集合（aggregable）。
5. 发布不可变轮次快照（publish），可封存（sealed）冻结。

## 核心状态机

- 聚合轮次：`preparing → receiving → validating → aggregable → sealed`
- 客户端更新：`new → valid | replay | forked → isolated`
- 模型节点：`candidate → confirmed | stale | conflicted`
- 轮次快照：`draft → publish → supersede`

## 标准命令

```bash
# 构建
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
# 校验
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...
# 一键自检（重启恢复）
go run ./cmd/fedlineage --smoke-test
# 长驻服务
go run ./cmd/fedlineage --addr :8080 --db fedlineage.db
```

## 目录结构

```
env/
├── cmd/fedlineage/main.go      # 入口（--addr/--db/--smoke-test）
├── internal/
│   ├── model/                  # 实体、错误、状态机、摘要
│   ├── store/                  # SQLite 持久化（建表 + CRUD）
│   ├── node/                   # 模型节点维护 + 谱系边 + 环检测
│   ├── round/                  # 轮次状态机与生命周期
│   ├── update/                 # 更新接收与幂等去重
│   ├── lineage/                # 父模型/形状校验、分叉检测、祖先路径
│   ├── aggregate/              # 可聚合集合计算、确认、审计
│   ├── snapshot/               # 不可变快照发布
│   ├── service/                # 编排层 + 自检
│   └── httpapi/                # HTTP 层（/api 前缀，30 个端点）
└── ...
```

## 持久化与重启恢复

使用纯 Go 驱动 modernc.org/sqlite（WAL 模式），所有实体落库。
`--smoke-test` 会在临时库上真实创建实体、关闭并重新打开数据库验证恢复。
更新 ID 为幂等键；封存轮次不接受修改；发布快照不可被重放覆盖。

## 并发与错误边界

- 客户端可并行上报；轮次关闭串行。
- 拒绝：参数摘要缺失、父轮次未来、更新 ID 冲突、封存轮次修改。
- 维度/形状不一致 → forked；相同 ID 重放 → replay；主动隔离 → isolated。
