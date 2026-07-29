package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/security/ruleengine"
	"code-agent-sentinel/internal/storage"
)

// ruleDomainFromPath 按请求路径前缀判断域:
// /api/intercept-rules* → intercept;否则(detect-rules*) → detect。
// 两域共用同一组 handler 方法,由路径前缀分流。用 HasPrefix 比 brief 原稿的
// Path[:19] 切片更稳健(不依赖固定长度,路径变长也不会越界)。
func ruleDomainFromPath(c *gin.Context) storage.Domain {
	if strings.HasPrefix(c.Request.URL.Path, "/api/intercept-rules") {
		return storage.DomainIntercept
	}
	return storage.DomainDetect
}

// boolToInt:Go 不允许 bool→int 隐式转换(SQLite dotall 列是 INTEGER)。
// ruleengine 包内有同名 helper(load_db.go)但未导出,api 包不可见,故在此重定义一份。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// registerRulesRoutes 注册规则管理路由(检测/拦截两域对称,同一组 handler 经路径前缀分流)。
// Task 11 的测试调用此方法注册路由到 server 的 router;Task 12 的 registerRoutes
// 也会调用此方法把规则路由并入主路由表(避免重复注册逻辑两处维护)。
//
// 路由清单(每域 8 个):
//   GET    /{domain}-rules            列表
//   GET    /{domain}-rules/:id        单条
//   POST   /{domain}-rules            新建 custom
//   PUT    /{domain}-rules/:id        更新 custom(builtin 409)
//   DELETE /{domain}-rules/:id        删除 custom(builtin 409)
//   PUT    /{domain}-rules/:id/enabled 启停(builtin/custom 都可)
//   POST   /{domain}-rules/:id/fork   fork builtin → custom
//   POST   /{domain}-rules/validate   校验草稿(不落库)
//
// 注意:Gin 路由匹配按注册顺序静态优先,validate 是 /detect-rules 下唯一的 POST 静态段,
// :id 是动态段——/detect-rules/validate 走 POST,会被 :id 匹配吗?Gin 对同一 method
// 的静态/动态段:静态优先。但本处 validate 与 :id 同为 POST /detect-rules/xxx,
// Gin panics 若同一 path 段位既有静态又有参数。故 validate 用 /detect-rules/validate
// 与 POST /detect-rules(无段)+ POST /detect-rules/:id/fork(两段)不冲突——
// validate 是单段 POST,fork 是两段 POST,二者段数不同不冲突。
// GET /detect-rules/:id 与 POST /detect-rules/validate 方法不同(method 树分离)不冲突。
func (s *Server) registerRulesRoutes(rg *gin.RouterGroup) {
	for _, prefix := range []string{"detect-rules", "intercept-rules"} {
		rg.GET("/"+prefix, s.getRules)
		rg.GET("/"+prefix+"/:id", s.getRule)
		rg.POST("/"+prefix, s.postRule)
		rg.PUT("/"+prefix+"/:id", s.putRule)
		rg.DELETE("/"+prefix+"/:id", s.deleteRule)
		rg.PUT("/"+prefix+"/:id/enabled", s.putRuleEnabled)
		rg.POST("/"+prefix+"/:id/fork", s.forkRule)
		rg.POST("/"+prefix+"/validate", s.validateRule)
	}
}

// getRules 列出某域全部规则(含 enabled 派生状态)。
func (s *Server) getRules(c *gin.Context) {
	domain := ruleDomainFromPath(c)
	if s.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "db_unavailable", "message": "规则库未初始化"}})
		return
	}
	stored, err := storage.ListRules(s.DB, domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	dtos := make([]ruleDTO, 0, len(stored))
	for _, st := range stored {
		dtos = append(dtos, s.storedToDTO(st, domain))
	}
	c.JSON(http.StatusOK, dtos)
}

// getRule 取单条。
func (s *Server) getRule(c *gin.Context) {
	domain := ruleDomainFromPath(c)
	id := c.Param("id")
	if s.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "db_unavailable", "message": "规则库未初始化"}})
		return
	}
	st, ok, err := storage.GetRule(s.DB, domain, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "rule not found"}})
		return
	}
	c.JSON(http.StatusOK, s.storedToDTO(st, domain))
}

// postRule 新建 custom 规则(校验后落库)。
func (s *Server) postRule(c *gin.Context) {
	domain := ruleDomainFromPath(c)
	if s.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "db_unavailable", "message": "规则库未初始化"}})
		return
	}
	var dto ruleDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": err.Error()}})
		return
	}
	// POST = 创建,不允许覆盖已有规则(含 builtin)。builtin id 冲突尤其须拒:
	// UpsertRule 的 ON CONFLICT(rule_id) DO UPDATE 会把 builtin 行的 source 改成 custom
	// 并覆盖其 match,静默禁用安全规则——违反"内置只读"铁律。更新走 PUT(PUT 对 builtin 返回 409)。
	// 镜像 forkRule 的冲突检查模式。即便针对 custom 也拒:create 重复 id 是冲突,改用 PUT 或换 id。
	if existing, ok, _ := storage.GetRule(s.DB, domain, dto.ID); ok {
		code := "id_conflict"
		msg := "规则 ID 已存在"
		if existing.Source == "builtin" {
			code = "builtin_readonly"
			msg = "内置规则 ID 已存在,请 fork 或换一个 ID"
		}
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": code, "message": msg}})
		return
	}
	if err := validateAndUpsertRule(s, domain, dto, "custom", ""); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_rule", "message": err.Error()}})
		return
	}
	st, _, _ := storage.GetRule(s.DB, domain, dto.ID)
	c.JSON(http.StatusOK, s.storedToDTO(st, domain))
}

// putRule 更新 custom 规则(内置返回 409)。
func (s *Server) putRule(c *gin.Context) {
	domain := ruleDomainFromPath(c)
	id := c.Param("id")
	if s.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "db_unavailable", "message": "规则库未初始化"}})
		return
	}
	var dto ruleDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": err.Error()}})
		return
	}
	dto.ID = id
	// 内置规则拒绝编辑(需 fork 为 custom 后再改)。
	existing, ok, _ := storage.GetRule(s.DB, domain, id)
	if ok && existing.Source == "builtin" {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "builtin_readonly", "message": "内置规则只读,请 fork 为自定义后编辑"}})
		return
	}
	if err := validateAndUpsertRule(s, domain, dto, "custom", ""); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_rule", "message": err.Error()}})
		return
	}
	st, _, _ := storage.GetRule(s.DB, domain, id)
	c.JSON(http.StatusOK, s.storedToDTO(st, domain))
}

// deleteRule 删除 custom 规则(内置返回 409,不存在返回 404)。
func (s *Server) deleteRule(c *gin.Context) {
	domain := ruleDomainFromPath(c)
	id := c.Param("id")
	if s.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "db_unavailable", "message": "规则库未初始化"}})
		return
	}
	existing, ok, _ := storage.GetRule(s.DB, domain, id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "rule not found"}})
		return
	}
	if existing.Source == "builtin" {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "builtin_readonly", "message": "内置规则不可删除"}})
		return
	}
	if err := storage.DeleteRule(s.DB, domain, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// putRuleEnabled 启停规则(builtin/custom 都可)。body: {enabled: bool}。
// builtin 规则不可删/改,但可禁用(override 表记录 enabled=0,运行时 ListRulesEnabled 过滤)。
func (s *Server) putRuleEnabled(c *gin.Context) {
	domain := ruleDomainFromPath(c)
	id := c.Param("id")
	if s.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "db_unavailable", "message": "规则库未初始化"}})
		return
	}
	var body enabledBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": err.Error()}})
		return
	}
	if _, ok, _ := storage.GetRule(s.DB, domain, id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "rule not found"}})
		return
	}
	if err := storage.SetOverride(s.DB, domain, id, body.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rule_id": id, "enabled": body.Enabled})
}

// forkRule 复制 builtin 为 custom(新 id)。只允许 fork builtin(custom 不可再 fork)。
// 实现说明:src 是 storage.StoredRule(builtin 行的快照),改 src.ID = new_id 后以
// source="custom" 调 UpsertRule——UpsertRule 的 source 参数会覆盖 src.Source 写入 db,
// 故 fork 行的 source 列为 "custom"、builtin_version 为 NULL(传 "" → builtinVersionOrNull)。
// 原 builtin 行 rule_id 不同,不受影响。
func (s *Server) forkRule(c *gin.Context) {
	domain := ruleDomainFromPath(c)
	id := c.Param("id")
	if s.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "db_unavailable", "message": "规则库未初始化"}})
		return
	}
	var body forkRuleBody
	if err := c.ShouldBindJSON(&body); err != nil || body.NewID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "new_id required"}})
		return
	}
	src, ok, _ := storage.GetRule(s.DB, domain, id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "rule not found"}})
		return
	}
	if src.Source != "builtin" {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "not_builtin", "message": "只能 fork 内置规则"}})
		return
	}
	if _, exists, _ := storage.GetRule(s.DB, domain, body.NewID); exists {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "id_conflict", "message": "new_id 已存在"}})
		return
	}
	// 复制为 custom:改 ID + source=custom(custom 行 builtin_version 存 NULL → 传 "")。
	src.ID = body.NewID
	if err := storage.UpsertRule(s.DB, domain, "custom", src, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	st, _, _ := storage.GetRule(s.DB, domain, body.NewID)
	c.JSON(http.StatusOK, s.storedToDTO(st, domain))
}

// validateRule 校验草稿(不落库)。body: ruleDTO。
// 返回 {valid: bool, errors: []string};校验失败仍返回 200(非错误响应),
// 便于前端区分"请求失败"与"规则不合法"。
func (s *Server) validateRule(c *gin.Context) {
	var dto ruleDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false, "errors": []string{err.Error()}})
		return
	}
	if errs := validateRuleDTO(dto); len(errs) > 0 {
		c.JSON(http.StatusOK, gin.H{"valid": false, "errors": errs})
		return
	}
	if _, err := dtoToRule(dto); err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false, "errors": []string{err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "errors": []string{}})
}

// validateAndUpsertRule:DTO → 基础校验 → dtoToRule(含 ruleengine.Validate 正则编译) →
// RuleToStoredRule → storage.UpsertRule。custom 行 builtin_version 传 ""(存 NULL)。
// version 参数保留给未来 builtin 行刷新场景(custom 永远传 "")。
func validateAndUpsertRule(s *Server, domain storage.Domain, dto ruleDTO, source, version string) error {
	if errs := validateRuleDTO(dto); len(errs) > 0 {
		return fmt.Errorf("校验失败: %v", errs)
	}
	r, err := dtoToRule(dto)
	if err != nil {
		return err
	}
	stored, err := ruleengine.RuleToStoredRule(r, source, version)
	if err != nil {
		return err
	}
	return storage.UpsertRule(s.DB, domain, source, stored, version)
}

// validateRuleDTO 做基础结构校验(非空必填);完整校验(正则编译、op 合法性、
// match 树递归)由 ruleengine.Validate 在 dtoToRule 内跑,失败转 error 返回。
func validateRuleDTO(dto ruleDTO) []string {
	var errs []string
	if dto.ID == "" {
		errs = append(errs, "id 不能为空")
	}
	if dto.Severity == "" {
		errs = append(errs, "severity 不能为空")
	}
	if dto.AssetType == "" {
		errs = append(errs, "asset_type 不能为空")
	}
	if len(dto.Match) == 0 {
		errs = append(errs, "match 不能为空")
	}
	return errs
}

// dtoToRule:ruleDTO → ruleengine.Rule(经 NewMatchNode 构造 MatchNode,与 YAML 加载路径同构)。
// 随后跑 ruleengine.Validate 编译正则 + 递归校验 match 树,失败返回错误(不落库)。
//
// 偏差说明(brief 修正 #4):brief 原稿用 ruleengine.RuleRow 承接 DTO 字段,但 RuleRow
// 的 match/paths 等是 JSON 文本列(MatchJSON/PathsJSON string),与 DTO 的 map/结构字段
// 不同构,直接赋值编译不过。DTO 字段与 ruleengine.Rule(非 RuleRow)一一对应(Match 是
// MatchNode、Paths 是 *PathFilter),故直接构造 Rule 经 NewMatchNode(dto.Match) 构造 MatchNode,
// 与 parseRuleYAML→UnmarshalYAML 产出同构,Validate 后行为与 YAML 加载等价(Task 7 等价性门)。
func dtoToRule(dto ruleDTO) (ruleengine.Rule, error) {
	r := ruleengine.Rule{
		ID:            dto.ID,
		Severity:      dto.Severity,
		AssetType:     dto.AssetType,
		Match:         ruleengine.NewMatchNode(dto.Match),
		Deobfuscation: dto.Deobfuscation,
		Dotall:        dto.Dotall,
		Paths:         dto.Paths,
		PostExclude:   dto.PostExclude,
		Remediation:   dto.Remediation,
		Description:   dto.Description,
		Metadata:      dto.Metadata,
	}
	if _, verrs := ruleengine.Validate([]ruleengine.Rule{r}); len(verrs) > 0 {
		return ruleengine.Rule{}, fmt.Errorf("%s", verrs[0].Reason)
	}
	return r, nil
}

// storedToDTO:storage.StoredRule → ruleDTO(含 enabled JOIN)。
// brief 修正 #1:改为 Server 方法用 s.DB 读 override(原稿用全局 globalDBForRead 错误)。
// 经 ruleengine.RowFromDBColumns + RowToRule 还原 Rule 的 map/结构字段(match/paths 等),
// 再映射到 DTO;Source/Enabled/CanEdit/Domain 是 DTO 派生字段。
func (s *Server) storedToDTO(st storage.StoredRule, domain storage.Domain) ruleDTO {
	row, err := ruleengine.RowFromDBColumns(st.ID, st.Severity, st.AssetType, st.MatchJSON, st.PathsJSON,
		st.Deobfuscation, boolToInt(st.Dotall), st.PostExclude, st.Remediation, st.Description, st.MetadataJSON)
	r := ruleengine.Rule{}
	if err == nil {
		r, _ = ruleengine.RowToRule(row)
	}
	enabled := true // 无 override = 默认启用(builtin/custom 均如此)
	if s.DB != nil {
		if e, exists, _ := storage.GetOverride(s.DB, domain, st.ID); exists {
			enabled = e
		}
	}
	// 从还原后的 Rule 取 map/结构字段(经 NewMatchNode→Raw 往返保持原始结构)。
	var matchMap map[string]any
	if r.Match.Raw() != nil {
		matchMap = r.Match.Raw()
	}
	return ruleDTO{
		ID:            st.ID,
		Severity:      st.Severity,
		AssetType:     st.AssetType,
		Match:         matchMap,
		Deobfuscation: r.Deobfuscation,
		Dotall:        st.Dotall,
		Paths:         r.Paths,
		PostExclude:   r.PostExclude,
		Remediation:   r.Remediation,
		Description:   r.Description,
		Metadata:      r.Metadata,
		Source:        st.Source,
		Enabled:       enabled,
		CanEdit:       st.Source == "custom",
		Domain:        string(domain),
	}
}
