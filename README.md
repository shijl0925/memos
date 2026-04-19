# memos

## usememos/memos `v0.1.0` 迁移到 `gin-ninja` 改造难度评估

结论：**中高难度（约 3~8 人天，受接口规模、测试覆盖率与历史技术债影响）**。

### 主要工作量来源

1. **路由层改造（中）**
   - 将原 `gin` 路由注册方式迁移到 `gin-ninja` 的分组+typed handler 风格。
   - 示例：`r.GET("/api/memo", handler)` 可迁移为 `ninja.Get(router, "/api/memo", listMemos)`，其中 `listMemos` 通常为 `func(ctx *ninja.Context, in *ListMemosRequest) (*ListMemosResponse, error)` 形式。
   - 需要逐个 API 校对 URL、Method、参数绑定和中间件挂载顺序。

2. **参数绑定与校验（中）**
   - 统一迁移 `ShouldBindJSON/Query/Uri` 等输入绑定写法。
   - 校验错误结构可能变化（如字段名、错误数组结构、HTTP 状态码），建议通过统一错误转换层维持旧响应格式（例如继续返回既有 `code/message` 结构）。

3. **中间件与上下文（中高）**
   - 认证、鉴权、CORS、日志、错误恢复等中间件链需要逐条验证。
   - `gin.Context` 读写习惯若变化，会影响 handler 逻辑和错误处理分支。

4. **文档与测试回归（中高）**
   - 若项目依赖 OpenAPI/Swagger，需要同步验证生成结果。
   - 建议针对登录、笔记 CRUD、标签/资源上传等核心路径做回归测试。

### 推荐最小风险迁移策略

1. 先保留原目录结构，仅替换路由注册入口（不改业务层）。
2. 分模块迁移（auth -> user -> memo -> resource），每模块迁移后立刻回归。
3. 保持响应体结构不变，先兼容再优化。
4. 最后统一整理中间件和错误码映射。
