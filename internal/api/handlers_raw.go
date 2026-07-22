package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
)

// rawResponse 返回单个文件的原始内容(文本)。
//
// 用于文件树里「无资产」的文件:点开时右侧详情读取其原始内容展示(plaintext/代码)。
// 配置资产(settings/memory/skill...)已有结构化资产,不走此接口。
type rawResponse struct {
	Path    string `json:"path"`    // 绝对路径(回显,便于前端定位)
	Name    string `json:"name"`    // 文件名
	Size    int64  `json:"size"`    // 字节数
	Content string `json:"content"` // 文本内容(二进制文件返回截断提示)
	IsText  bool   `json:"is_text"` // 是否按文本读取(非文本→content 为提示)
}

// maxRawBytes 限制单次读取大小,防读超大文件(如 history.jsonl 可能数 MB)撑爆响应。
const maxRawBytes = 512 * 1024 // 512KB

// resolveTreeRoots 返回所有合法的「树根」绝对路径:选中 agent 的全局根(eng.ClaudeDir
// = <home>/.claude)+ 该 agent 已知项目的 .claude 与项目根。
// /api/raw 仅允许读这些根之下的文件,防越权遍历到任意路径。
//
// 用选中 agent 的 Engine 而非 s.Agents[0] / s.Engine:?agent=b 时应返回 b 的根 +
// b 的项目(s.Agents[0].RootDir 是 agent a 的根,s.Engine 也是 a 的 Engine,会把
// 根与项目列表错配——同 Task 7 getTree / Task 8 getProject 的 bug 模式)。
func (s *Server) resolveTreeRoots(eng *configengine.Engine) []string {
	var roots []string
	if eng != nil && eng.ClaudeDir != "" {
		roots = append(roots, eng.ClaudeDir)
	}
	if eng != nil {
		projects, _ := eng.ListProjects()
		for _, p := range projects {
			roots = append(roots, filepath.Join(p.Path, ".claude"))
			// .mcp.json 在项目根(非 .claude 下),也允许读项目根目录下的文件。
			roots = append(roots, p.Path)
		}
	}
	return roots
}

// getRaw 读取单个文件原始内容。
//
// 安全校验:
//  1. path 必须在某合法树根(选中 agent 的全局 .claude / 项目 .claude / 项目根)之下——
//     filepath.Rel + 不含 ".." ——防 ../../../etc/passwd 越权;
//  2. 必须是普通文件(非目录、非设备);
//  3. 读取上限 maxRawBytes;
//  4. 二进制文件(含 NUL 字节)不返回内容,标记 is_text=false。
func (s *Server) getRaw(c *gin.Context) {
	p := c.Query("path")
	if p == "" {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", "path required"))
		return
	}
	eng, _, err := s.engineForQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("unknown_agent", err.Error()))
		return
	}
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	// 必须落在某合法树根之下。
	root := s.containingRoot(eng, abs)
	if root == "" {
		c.JSON(http.StatusForbidden, errorBody("out_of_root", "path is not under any known tree root"))
		return
	}
	fi, err := os.Stat(abs)
	if err != nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", err.Error()))
		return
	}
	if fi.IsDir() {
		c.JSON(http.StatusBadRequest, errorBody("is_dir", "path is a directory"))
		return
	}
	// 读最多 maxRawBytes+1(+1 用于判断是否截断)。
	f, err := os.Open(abs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("read_failed", err.Error()))
		return
	}
	defer f.Close()
	buf := make([]byte, maxRawBytes+1)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		c.JSON(http.StatusInternalServerError, errorBody("read_failed", err.Error()))
		return
	}
	truncated := n > maxRawBytes
	if truncated {
		n = maxRawBytes
	}
	data := buf[:n]
	// 二进制检测:含 NUL 字节视为非文本。
	isText := !strings.ContainsRune(string(data), 0)
	resp := rawResponse{
		Path:   abs,
		Name:   filepath.Base(abs),
		Size:   fi.Size(),
		IsText: isText,
	}
	if isText {
		content := string(data)
		if truncated {
			content += "\n…(已截断,文件超过 512KB)"
		}
		resp.Content = content
	} else {
		resp.Content = "二进制文件,不展示内容"
	}
	c.JSON(http.StatusOK, resp)
}

// containingRoot 返回 abs 所属的合法树根(绝对路径);都不属于返回 ""。
// 用 filepath.Rel 判断:rel 不以 ".." 开头且不含 ".." 段 → abs 在 root 之下。
func (s *Server) containingRoot(eng *configengine.Engine, abs string) string {
	for _, root := range s.resolveTreeRoots(eng) {
		rel, err := filepath.Rel(filepath.Clean(root), abs)
		if err != nil {
			continue
		}
		if rel == "." {
			return root
		}
		// rel 不得以 ".." 开头(表示 abs 在 root 之外)。
		if strings.HasPrefix(rel, "..") {
			continue
		}
		return root
	}
	return ""
}
