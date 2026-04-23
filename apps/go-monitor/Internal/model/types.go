package model

import (
	"encoding/json"
	"time"
)

type Repository struct {
	ID             int64  `json:"id"`
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	FullName       string `json:"full_name"`
	DefaultBranch  string `json:"default_branch,omitempty"`
	InstallationID int64  `json:"installation_id"`
	HTMLURL        string `json:"html_url,omitempty"`
}

type NormalizedEvent struct {
	DeliveryID        string          `json:"delivery_id"`
	Event             string          `json:"event"`
	Action            string          `json:"action,omitempty"`
	Repository        Repository      `json:"repository"`
	InstallationID    int64           `json:"installation_id"`
	Ref               string          `json:"ref,omitempty"`
	Branch            string          `json:"branch,omitempty"`
	Before            string          `json:"before,omitempty"`
	After             string          `json:"after,omitempty"`
	Created           bool            `json:"created,omitempty"`
	Deleted           bool            `json:"deleted,omitempty"`
	PullRequestNumber int             `json:"pull_request_number,omitempty"`
	HeadSHA           string          `json:"head_sha,omitempty"`
	BaseRef           string          `json:"base_ref,omitempty"`
	WorkflowName      string          `json:"workflow_name,omitempty"`
	CheckName         string          `json:"check_name,omitempty"`
	Status            string          `json:"status,omitempty"`
	Conclusion        string          `json:"conclusion,omitempty"`
	ReleaseTag        string          `json:"release_tag,omitempty"`
	SenderLogin       string          `json:"sender_login,omitempty"`
	ReceivedAt        time.Time       `json:"received_at"`
	Raw               json.RawMessage `json:"raw,omitempty"`
}

type CommitRef struct {
	SHA    string `json:"sha"`
	Branch string `json:"branch,omitempty"`
	Ref    string `json:"ref,omitempty"`
}

type DirectoryNode struct {
	Path   string `json:"path"`
	Parent string `json:"parent,omitempty"`
}

type FileNode struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Language string `json:"language,omitempty"`
	Size     int64  `json:"size,omitempty"`
	SHA      string `json:"sha,omitempty"`
}

type Dependency struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Scope  string `json:"scope,omitempty"`
}

type Workflow struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type TreeSnapshot struct {
	Files       []FileNode      `json:"files"`
	Directories []DirectoryNode `json:"directories"`
}

type RepoSnapshot struct {
	Repo         Repository        `json:"repo"`
	Commit       CommitRef         `json:"commit"`
	TreeSHA      string            `json:"tree_sha,omitempty"`
	Tree         TreeSnapshot      `json:"tree"`
	Dependencies []Dependency      `json:"dependencies"`
	Workflows    []Workflow        `json:"workflows"`
	EventsWindow []NormalizedEvent `json:"events_window,omitempty"`
	Metadata     map[string]any    `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

type RepoDiff struct {
	AddedFiles          []FileNode      `json:"added_files"`
	RemovedFiles        []FileNode      `json:"removed_files"`
	ModifiedFiles       []FileNode      `json:"modified_files"`
	AddedDirectories    []DirectoryNode `json:"added_directories"`
	RemovedDirectories  []DirectoryNode `json:"removed_directories"`
	AddedDependencies   []Dependency    `json:"added_dependencies"`
	RemovedDependencies []Dependency    `json:"removed_dependencies"`
	AddedWorkflows      []Workflow      `json:"added_workflows"`
	RemovedWorkflows    []Workflow      `json:"removed_workflows"`
}

type StoredSnapshot struct {
	ID        string       `json:"id"`
	RepoID    int64        `json:"repo_id"`
	CommitSHA string       `json:"commit_sha"`
	Branch    string       `json:"branch,omitempty"`
	TreeSHA   string       `json:"tree_sha,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	Snapshot  RepoSnapshot `json:"snapshot"`
}

type VizNode struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Kind     string         `json:"kind"`
	Parent   string         `json:"parent,omitempty"`
	Status   string         `json:"status,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type VizEdge struct {
	ID       string         `json:"id"`
	Source   string         `json:"source"`
	Target   string         `json:"target"`
	Kind     string         `json:"kind"`
	Status   string         `json:"status,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type VizModel struct {
	Kind         string         `json:"kind"`
	Repo         Repository     `json:"repo"`
	ModelVersion int64          `json:"model_version"`
	Nodes        []VizNode      `json:"nodes"`
	Edges        []VizEdge      `json:"edges"`
	LayoutHints  map[string]any `json:"layout_hints,omitempty"`
}

type VizPatchOp struct {
	Op        string         `json:"op"`
	ID        string         `json:"id,omitempty"`
	Kind      string         `json:"kind,omitempty"`
	Node      *VizNode       `json:"node,omitempty"`
	Edge      *VizEdge       `json:"edge,omitempty"`
	Set       map[string]any `json:"set,omitempty"`
	Animation string         `json:"animation,omitempty"`
}

type VizPatch struct {
	Type         string       `json:"type"`
	RepoID       int64        `json:"repo_id"`
	PatchID      string       `json:"patch_id,omitempty"`
	FromVersion  int64        `json:"from_version,omitempty"`
	ToVersion    int64        `json:"to_version"`
	ModelVersion int64        `json:"model_version"`
	Changes      []VizPatchOp `json:"changes"`
	Event        string       `json:"event,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

type CompileRequest struct {
	Snapshot      *RepoSnapshot    `json:"snapshot,omitempty"`
	PreviousModel *VizModel        `json:"previous_model,omitempty"`
	Diff          *RepoDiff        `json:"diff,omitempty"`
	Event         *NormalizedEvent `json:"event,omitempty"`
	Context       map[string]any   `json:"context,omitempty"`
}

type CompileResponse struct {
	ModelVersion int64          `json:"model_version"`
	CatlabModel  map[string]any `json:"catlab_model"`
	PetriModel   map[string]any `json:"petri_model"`
	VizModel     VizModel       `json:"viz_model"`
	Patch        VizPatch       `json:"patch"`
	Warnings     []string       `json:"warnings,omitempty"`
}
