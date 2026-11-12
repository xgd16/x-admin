package xadmin

import (
	"encoding/json"
	"io/fs"
	"time"
)

const (
	// ToolVersion 是工具自身的版本，和框架版本（Release tag）无关。
	ToolVersion = "0.1.0"

	// DefaultRepo 是框架上游仓库。
	DefaultRepo = "xgd16/x-admin"

	// StatePath 记录当前项目已经同步到的框架版本。
	// 放在仓库里提交，方便没有完整 fork 历史（Use this template）的项目升级。
	StatePath = ".x-admin/framework.json"

	// VersionPath 是可见的框架版本文件。
	// 比 .x-admin/framework.json 更不容易在复制源码时被漏掉；
	// 新 git init 的项目优先读它来确定升级基准。
	VersionPath = "FRAMEWORK_VERSION"
)

// File 是三方合并中的一份文件内容。
type File struct {
	Data []byte
	Mode fs.FileMode
}

// Tree 是 path -> File 的映射，path 统一使用斜杠分隔的相对路径。
type Tree map[string]File

// ChangeKind 描述一个文件在升级计划中的处理方式。
type ChangeKind string

const (
	// ChangeUpdate 表示用上游内容覆盖，或三方合并后写入。
	ChangeUpdate ChangeKind = "update"
	// ChangeAdd 表示上游新增文件。
	ChangeAdd ChangeKind = "add"
	// ChangeDelete 表示上游删除了文件，且本地没有改过。
	ChangeDelete ChangeKind = "delete"
	// ChangeKeep 表示上游未改或已删除，但本地有修改，保留本地。
	ChangeKeep ChangeKind = "keep"
	// ChangeSame 表示三方内容一致，无需处理。
	ChangeSame ChangeKind = "same"
	// ChangeConflict 表示需要人工处理。
	ChangeConflict ChangeKind = "conflict"
)

// Change 是一个文件的升级动作。
type Change struct {
	Path   string     `json:"path"`
	Kind   ChangeKind `json:"kind"`
	Reason string     `json:"reason,omitempty"`
	// UpstreamOnly 表示不覆盖本地文件，而是把上游版本写到 path+".upstream"。
	UpstreamOnly bool `json:"upstream_only,omitempty"`
	// Data 是 update/add/conflict 要写入的内容，不参与 JSON 输出。
	Data []byte      `json:"-"`
	Mode fs.FileMode `json:"-"`
}

// Summary 是升级计划的文件统计。
type Summary struct {
	Update   int `json:"update"`
	Add      int `json:"add"`
	Delete   int `json:"delete"`
	Keep     int `json:"keep"`
	Same     int `json:"same"`
	Conflict int `json:"conflict"`
}

// MigrationIssue 是迁移编号冲突或顺序风险。
type MigrationIssue struct {
	// Severity 取值 error 或 warning。
	Severity string `json:"severity"`
	Number   int    `json:"number"`
	Local    string `json:"local,omitempty"`
	Upstream string `json:"upstream,omitempty"`
	Message  string `json:"message"`
}

// ConfigChange 是上游新增、但本地 configs/config.yaml 还没有的配置项。
type ConfigChange struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Plan 是一次升级的完整计划。
type Plan struct {
	Repo        string           `json:"repo"`
	WorkDir     string           `json:"work_dir"`
	FromRef     string           `json:"from_ref"`
	FromCommit  string           `json:"from_commit,omitempty"`
	FromSource  string           `json:"from_source,omitempty"`
	ToRef       string           `json:"to_ref"`
	ToCommit    string           `json:"to_commit,omitempty"`
	Changes     []Change         `json:"changes"`
	Migrations  []MigrationIssue `json:"migration_issues,omitempty"`
	ConfigAdded []ConfigChange   `json:"config_added,omitempty"`
	Stats       Summary          `json:"summary"`
}

// ComputeSummary 统计各类变更数量。
func (p *Plan) ComputeSummary() Summary {
	var s Summary
	for _, ch := range p.Changes {
		switch ch.Kind {
		case ChangeUpdate:
			s.Update++
		case ChangeAdd:
			s.Add++
		case ChangeDelete:
			s.Delete++
		case ChangeKeep:
			s.Keep++
		case ChangeSame:
			s.Same++
		case ChangeConflict:
			s.Conflict++
		}
	}
	return s
}

// HasWork 表示计划中是否有需要写入工作区的动作。
func (p *Plan) HasWork() bool {
	for _, ch := range p.Changes {
		switch ch.Kind {
		case ChangeUpdate, ChangeAdd, ChangeDelete, ChangeConflict:
			return true
		}
	}
	return false
}

// HasConflicts 表示是否存在需要人工处理的文件。
func (p *Plan) HasConflicts() bool {
	for _, ch := range p.Changes {
		if ch.Kind == ChangeConflict {
			return true
		}
	}
	return false
}

// HasMigrationErrors 表示是否存在迁移编号冲突。
func (p *Plan) HasMigrationErrors() bool {
	for _, issue := range p.Migrations {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

// State 是 .x-admin/framework.json 的结构。
type State struct {
	Repo      string    `json:"repo"`
	Ref       string    `json:"ref"`
	Commit    string    `json:"commit,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// MarshalJSON 让 Plan 序列化时自动带上统计。
func (p *Plan) MarshalJSON() ([]byte, error) {
	type alias Plan
	p.Stats = p.ComputeSummary()
	return json.Marshal((*alias)(p))
}
